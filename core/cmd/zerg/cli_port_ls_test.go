// cli_port_ls_test.go —— P0-7 `zerg port ls` 的**真二进制**判据（缺口-命令面-20260921 §十一）。
//
// 判据的两头都用**自己造的监听**钉住（不依赖本机跑没跑主控）：
// 起一个真 LISTEN ⇒ 有名有姓地读到属主（退 0）；关掉它 ⇒ 同一端口「没有属主」（退 8）。
// 这就是「有进程在应答 ≠ 我的进程在应答」这条血泪判据的机械落点。
package main_test

import (
	"encoding/json"
	"net"
	"strconv"
	"testing"
)

func TestPortLs_OwnerThenNoOwner(t *testing.T) {
	bin := zergBinary(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("起临时监听：%v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ps := strconv.Itoa(port)

	rc, out, errb := execCase(t, bin, "invalid-ignored", "port", "ls", ps, "--json", "port,pid,path")
	if rc != 0 {
		t.Fatalf("有属主 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []struct {
			Port string `json:"port"`
			PID  string `json:"pid"`
			Path string `json:"path"`
		} `json:"items"`
		Meta struct{ Count int } `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	if env.Meta.Count == 0 {
		t.Fatalf("监听在 lsof 里读不到属主（本机 lsof 能力问题）：%q", out)
	}
	if env.Items[0].Port != ps {
		t.Errorf("端口列不对：want %s got %+v", ps, env.Items[0])
	}
	if env.Items[0].PID == "" {
		t.Errorf("pid 列为空（属主读到了却没给 pid）：%+v", env.Items[0])
	}
	if env.Items[0].Path == "" {
		t.Errorf("在跑件路径列为空：%+v", env.Items[0])
	}
	t.Logf("属主：port=%s pid=%s path=%s", env.Items[0].Port, env.Items[0].PID, env.Items[0].Path)

	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	rc, _, errb = execCase(t, bin, "invalid-ignored", "port", "ls", ps)
	if rc != 8 {
		t.Errorf("指名问、没人听 ⇒ 退 8（**取不到属主**，不是 0），得到 %d · stderr=%s", rc, errb)
	}
}

func TestPortLs_UsageErrors(t *testing.T) {
	bin := zergBinary(t)
	for _, argv := range [][]string{
		{"port", "ls", "99999"},
		{"port", "ls", "abc"},
		{"port", "ls", "0"},
		{"port", "ls", "--nosuchflag-zz"},
	} {
		rc, _, _ := execCase(t, bin, "x", argv...)
		if rc != 2 {
			t.Errorf("%v ⇒ 退 2，得到 %d", argv, rc)
		}
	}
}
