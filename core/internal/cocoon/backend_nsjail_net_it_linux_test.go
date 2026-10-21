//go:build linux

// backend_nsjail_net_it_linux_test.go — T5 的真机验收：**茧默认断网**。
//
// 三格（都在真 nsjail 里跑，不是参数层断言）：
//
//	① 缺省（不写 [茧].net）⇒ 茧内 curl 外网失败、DNS 失败，但**茧内自己的 127.0.0.1:<port> 能通**
//	   —— 第三句是这条的**正控**：没有它，「curl 失败」也可能只是「茧里没 curl / 全都不通」。
//	② net = "host" ⇒ 茧内 curl 外网可通（至少 TCP 连上），回环照旧。
//	③ 负控：给了 [[端口]] 却没写 net=host ⇒ 解析期就拒，文案点名 net=host（✗ 不静默跳过）。
//
// 起法（本机无 Linux 时用特权容器）：
//
//	docker exec <容器> env ZERG_COCOON_IT_NET=1 ./cocoon.test -test.run NsjailNetIsolation -test.v
//
// ★ 不换根档（不给 [茧].rootfs）：网络档与 rootfs 正交，这一格只验网络；
//
//	换根那四条在 backend_nsjail_rootfs_it_linux_test.go 里（那格与网络无关，走缺省 none）。
package cocoon

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// netITScript — 茧内一次性脚本：先把回环服务起起来（正控），再探外网与 DNS。
//
// 逐行读 `KEY=value`：本格不解析 shell 的退出码，只认这三个数（把「跑没跑到底」与
// 「探到什么」分开，报告里能同时看见）。python3 / curl / getent 都来自不换根档的宿主根。
const netITScript = `
python3 -m http.server 18080 --bind 127.0.0.1 --directory /tmp >/dev/null 2>&1 &
p=$!
sleep 1.5
code=$(curl -sS --max-time 3 -o /dev/null -w '%{http_code}' http://127.0.0.1:18080/ 2>/dev/null)
echo "LOOP=$code"
curl -sS --max-time 3 -o /dev/null http://1.1.1.1 2>/dev/null
echo "NET_RC=$?"
getent hosts example.com >/dev/null 2>&1
echo "DNS_RC=$?"
kill $p 2>/dev/null
exit 0
`

// netITField — 从脚本输出里取 `KEY=value`（取不到返回空串）。
func netITField(out, key string) string {
	for _, ln := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(ln), key+"="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func TestNsjailNetIsolationIntegration(t *testing.T) {
	if os.Getenv("ZERG_COCOON_IT_NET") != "1" {
		t.Skip("未设 ZERG_COCOON_IT_NET=1：本格只在 linux 真机/特权容器里跑（真 nsjail 建 netns）")
	}
	if os.Geteuid() != 0 {
		t.Skip("需要 root：nsjail 建 netns / cgroup 都要它")
	}
	if _, err := exec.LookPath("nsjail"); err != nil {
		t.Skipf("找不到 nsjail（%v）", err)
	}

	ctx := context.Background()
	be := NewNsjailBackend()
	be.BaseDir = t.TempDir()

	cases := []struct {
		name       string
		net        string // 契约里写的值（""= 不写这个键）
		wantOnline bool
	}{
		{"default_none", "", false},
		{"explicit_none", NetNone, false},
		{"host", NetHost, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := &Spec{File: SpecFileName, Cocoon: CocoonSpec{
				Name: "net-it", Image: "debian:bookworm", Net: tc.net,
			}}
			if got := spec.Cocoon.NetMode(); tc.net == "" && got != NetNone {
				t.Fatalf("缺省 NetMode() = %q，want %q", got, NetNone)
			}
			inst, err := be.Provision(ctx, spec)
			if err != nil {
				t.Fatalf("Provision: %v", err)
			}
			t.Cleanup(func() { _ = be.Destroy(ctx, inst) })

			res, err := be.Exec(ctx, inst, []string{"/bin/sh", "-c", netITScript},
				ExecOpts{Timeout: 90 * time.Second})
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			out := string(res.Stdout)
			t.Logf("茧内观测：stdout=%q stderr=%q rc=%d", out, res.Stderr, res.RC)

			// 正控（**两种档都必须成立**）：茧内回环照样能用。
			// 没有它，「外网不通」也可能只是「茧里什么都没有」的假象。
			if got := netITField(out, "LOOP"); got != "200" {
				t.Fatalf("茧内回环应当可通（LOOP=200），得到 LOOP=%q\nstdout=%s\nstderr=%s", got, out, res.Stderr)
			}
			netRC := netITField(out, "NET_RC")
			dnsRC := netITField(out, "DNS_RC")
			if netRC == "" || dnsRC == "" {
				t.Fatalf("脚本没跑到底（NET_RC=%q DNS_RC=%q）\nstdout=%s\nstderr=%s", netRC, dnsRC, out, res.Stderr)
			}

			if tc.wantOnline {
				// net=host：共享宿主 netns ⇒ 出网与 DNS 都该通（TCP 连上即算，301/200 都行）。
				if netRC != "0" {
					t.Fatalf("net=host 时茧内应当能连外网，得到 NET_RC=%s（宿主机自己通不通？）\nstderr=%s", netRC, res.Stderr)
				}
				if dnsRC != "0" {
					t.Fatalf("net=host 时茧内 DNS 应当可用，得到 DNS_RC=%s", dnsRC)
				}
				return
			}
			// net=none（缺省 / 显式）：**出不去**。
			if netRC == "0" {
				t.Fatalf("net=none 时茧内竟能出网（NET_RC=0）—— 默认断网没生效\nstdout=%s", out)
			}
			if dnsRC == "0" {
				t.Fatalf("net=none 时茧内 DNS 竟可用（DNS_RC=0）—— 独立 netns 没建成")
			}
		})
	}

	// ③ 负控：[[端口]] 与 net 的互斥必须是**解析期**判据（✗ 不是「open 时才拒」）。
	t.Run("ports_require_host_net", func(t *testing.T) {
		src := `[茧]
name  = "net-it"
image = "debian:bookworm"

[[服务]]
name  = "api"
entry = ["/bin/sh", "-c", "true"]

[[端口]]
inner = 18080

[资源]
cpu = 1
mem_gb = 1
disk_gb = 1
`
		_, perr := ParseSpec([]byte(src))
		if perr == nil {
			t.Fatal("[[端口]] + 缺省 net=none 应当在解析期拒执，却解析成功")
		}
		se, ok := AsSpecError(perr)
		if !ok {
			t.Fatalf("错误不是 *SpecError：%T", perr)
		}
		if !strings.Contains(se.Msg, "net=host") {
			t.Fatalf("负控文案必须点名 net=host（人要知道怎么改）：%q", se.Msg)
		}
		t.Logf("负控文案：%v", perr)
	})
}
