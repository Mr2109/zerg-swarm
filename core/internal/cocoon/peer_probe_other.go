//go:build !linux && !darwin

// peer_probe_other.go — 其余平台：**没有**可用的对端身份机制 ⇒ none（诚实退让）。
//
// ★ v3 §F 的表里 Windows HvSocket 是 weak 且"传输层无防伪"；那一档要靠 `SOCKADDR_HV`
// 的 {VmId,ServiceId}，属于后端件的活，本层不假装能取。取不到 ⇒ none，
// 授权就完全落在令牌上（v3 §A：判据 7 不依赖任何平台能力）。
package cocoon

// platformPeerProbe — 本平台没有探针。
func platformPeerProbe(int) (PeerIdentityRecord, bool) {
	return nonePeer("unsupported-platform"), false
}
