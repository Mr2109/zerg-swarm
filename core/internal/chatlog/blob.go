package chatlog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// blobsDir —— 外置内容目录（<存储目录>/blobs）。
func (s *Store) blobsDir() string { return filepath.Join(s.dir, BlobsDirName) }

// putBlob —— 内容寻址落盘：返回 (sha256 hex, ref)。ref 相对存储目录（blobs/<sha>.bin）。
//   - 已存在 ⇒ 读回核 sha（防碰撞 / 防损坏），核实即复用（幂等）。
//   - 新写 ⇒ 临时件 + 写后校验 + rename（原子可见）。
func (s *Store) putBlob(content string) (string, string, error) {
	data := []byte(content)
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	ref := BlobsDirName + "/" + sha + ".bin"
	p := filepath.Join(s.dir, filepath.FromSlash(ref))

	if err := os.MkdirAll(s.blobsDir(), 0o755); err != nil {
		return "", "", fmt.Errorf("chatlog: 创建外置目录失败: %w", err)
	}
	if old, err := os.ReadFile(p); err == nil {
		got := sha256.Sum256(old)
		if hex.EncodeToString(got[:]) != sha {
			return "", "", fmt.Errorf("%w：%s", ErrBlobCorrupt, ref)
		}
		return sha, ref, nil
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		os.Remove(tmp)
		return "", "", fmt.Errorf("chatlog: 写外置内容失败: %w", err)
	}
	back, err := os.ReadFile(tmp)
	if err != nil {
		os.Remove(tmp)
		return "", "", fmt.Errorf("chatlog: 回读外置内容失败: %w", err)
	}
	got := sha256.Sum256(back)
	if hex.EncodeToString(got[:]) != sha {
		os.Remove(tmp)
		return "", "", fmt.Errorf("%w：写后校验失败 %s", ErrBlobCorrupt, ref)
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return "", "", fmt.Errorf("chatlog: 落外置内容失败: %w", err)
	}
	return sha, ref, nil
}

// Blob —— 读外置内容原始字节（按 ref）。
func (s *Store) Blob(ref string) ([]byte, error) {
	if ref == "" {
		return nil, fmt.Errorf("chatlog: blob_ref 为空")
	}
	p := filepath.Join(s.dir, filepath.FromSlash(ref))
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("chatlog: 读外置内容失败: %w", err)
	}
	return b, nil
}

// VerifyBlob —— 读外置内容并按登记的 sha256 核对（sha 可核）。
func (s *Store) VerifyBlob(ref, wantSHA string) error {
	b, err := s.Blob(ref)
	if err != nil {
		return err
	}
	got := sha256.Sum256(b)
	if hex.EncodeToString(got[:]) != wantSHA {
		return fmt.Errorf("%w：%s（登记 %s · 实读 %s）", ErrBlobCorrupt, ref, wantSHA, hex.EncodeToString(got[:]))
	}
	return nil
}

// Content —— 取事件正文：外置则读盘并按登记的 sha256 核（✗ 不信任缓存 / ✗ 不跳过校验）。
func (s *Store) Content(ev Event) (string, error) {
	if ev.BlobRef == "" {
		return ev.Content, nil
	}
	if err := s.VerifyBlob(ev.BlobRef, ev.SHA256); err != nil {
		return "", err
	}
	b, err := s.Blob(ev.BlobRef)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
