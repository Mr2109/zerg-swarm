package compress

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 余量闸死循环的回归（本闸曾整批跑批闷死：eval 侧 2048/1024 档 10s 超时）。
//
// 病根：带 raw_ptr 的段被卸成占位符后 Tokens 仍 >0、段也仍在窗口里，
// 于是每轮都被选成「最大候选」再卸一次 —— 占位符里的数字越缩越小，token 几乎不降，
// 而循环体不看 ctx、也不看有没有进展。
//
// 这里的三条测试分别钉住三处防呆：ctx 可取消、unload 幂等、进展检测。

// ① unload 幂等：带指针段卸一次就到位，再卸必须返回 false（否则就是空转的燃料）。
func TestUnloadIdempotentRawPtr(t *testing.T) {
	s := &Segment{Kind: KindObservation, Text: strings.Repeat("x", 6000), RawPtr: "/tmp/raw-1.txt", Source: "t"}
	if !unload(s) {
		t.Fatal("首次卸载应返回 true")
	}
	first, firstTok := s.Text, s.Tokens
	if firstTok <= 0 || firstTok >= 1500 {
		t.Fatalf("占位符 token 不合理：%d", firstTok)
	}
	changed := 0
	for i := 0; i < 8; i++ {
		if unload(s) {
			changed++
			continue
		}
		break
	}
	if t.Failed() || changed > 2 {
		t.Fatalf("占位符改写次数异常：%d（应很快收敛）", changed)
	}
	if !strings.Contains(s.Text, "/tmp/raw-1.txt") || s.Tokens != EstimateTokens(s.Text) {
		t.Fatalf("收敛后形态不对：text=%q tokens=%d", s.Text, s.Tokens)
	}
	if !strings.Contains(first, "/tmp/raw-1.txt") {
		t.Fatalf("首次卸载没留指针：%q", first)
	}
	// 收敛后必须稳定：再卸 3 次也不许再改
	for i := 0; i < 3; i++ {
		if unload(s) {
			t.Fatalf("收敛后再卸载仍返回 true（第 %d 次）：%q", i+1, s.Text)
		}
	}
}

// ② unload 幂等：无指针段折叠一次后收敛，不许无限「保头尾」。
func TestUnloadIdempotentFolded(t *testing.T) {
	s := &Segment{Kind: KindObservation, Text: strings.Repeat("y", 5000), Source: "t"}
	if !unload(s) {
		t.Fatal("首次折叠应返回 true")
	}
	if !strings.Contains(s.Text, "已折叠") {
		t.Fatalf("折叠标记缺失：%q", s.Text)
	}
	n := 0
	for unload(s) {
		n++
		if n > 4 {
			t.Fatalf("折叠不收敛：%d 次仍在改", n)
		}
	}
	if !strings.Contains(s.Text, "已折叠") || s.Tokens != EstimateTokens(s.Text) {
		t.Fatalf("收敛后形态不对：text=%q tokens=%d", s.Text, s.Tokens)
	}
}

// ③ 余量闸必须在有限步内收手：卸完「可卸的唯一一条」后，窗口仍在线以上
// （钉住段与最近 N 条不可动），循环不许再转 —— 这就是修前 2048/1024 档卡死的地方。
func TestCompressHeadroomLoopTerminates(t *testing.T) {
	for _, budget := range []int{2048, 1024} {
		budget := budget
		t.Run(strconv.Itoa(budget), func(t *testing.T) {
			segs := []Segment{
				// 钉住的走查结论（不可卸载，且大到让累计永远在线以上）
				seg(KindTrajectory, strings.Repeat("p", 12000), true, ""),
				// 可卸载：带指针的工具输出（单条闸先卸，余量闸不许再「卸」它）
				seg(KindObservation, strings.Repeat("o", 6000), false, "/tmp/raw-1.txt"),
				seg(KindObservation, strings.Repeat("q", 6000), false, "/tmp/raw-2.txt"),
				// 不可卸载：无指针且短于保头尾门槛（折叠也拿不到 token）
				seg(KindObservation, strings.Repeat("s", 400), false, ""),
			}
			pol := DefaultPolicy(budget)

			type res struct {
				out []Segment
				err error
			}
			done := make(chan res, 1)
			go func() {
				out, _, err := (&PrunerFree{}).Compress(context.Background(), segs, pol)
				done <- res{out, err}
			}()
			select {
			case r := <-done:
				if r.err != nil {
					t.Fatalf("Compress: %v", r.err)
				}
				total := TotalTokens(r.out)
				headroomLimit := int(float64(budget) * pol.HeadroomPct)
				t.Logf("budget=%d headroom线=%d 修后 after=%d（>线是应然：钉住段不可动）", budget, headroomLimit, total)
				if total <= 0 {
					t.Fatal("after=0：把窗口掏空了")
				}
				if !strings.Contains(r.out[1].Text, "/tmp/raw-1.txt") {
					t.Fatalf("可卸段没卸：%q", r.out[1].Text)
				}
				if strings.Contains(r.out[0].Text, "已卸载") || strings.Contains(r.out[0].Text, "已折叠") {
					t.Fatal("钉住的段被改了")
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("budget=%d：5s 内没终止 —— 余量闸又在同一段上反复卸载", budget)
			}
		})
	}
}

// ④ ctx 一取消就得收手：循环体每轮都要看 ctx（不是只在进函数时看一次）。
func TestCompressHeadroomLoopHonorsCancel(t *testing.T) {
	segs := []Segment{
		// 每条都在单条闸以下（perOut=250），只能由余量闸处理 ⇒ 必然进 ② 的循环体
		seg(KindObservation, strings.Repeat("a", 800), false, "/tmp/r1.txt"),
		seg(KindObservation, strings.Repeat("b", 800), false, "/tmp/r2.txt"),
		seg(KindObservation, strings.Repeat("c", 800), false, "/tmp/r3.txt"),
		seg(KindTrajectory, strings.Repeat("t", 400), true, ""),
	}
	pol := DefaultPolicy(1000) // perOut=250/token，headroom=700 ⇒ 累计 600+100 撞线
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := (&PrunerFree{}).Compress(ctx, segs, pol)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ctx 已取消却返回 nil —— 循环体没看 ctx")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ctx 取消后 3s 仍在循环里")
	}
}
