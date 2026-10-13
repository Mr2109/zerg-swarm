package chatlog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
)

// ─────────────────────────────────────────────────────────────────────────────
// 对话提示装配 · 前缀纪律（波4 · 压缩片 · 逻辑不接命令面）
//
// 设计真源：Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md §二.7 A5/A6/B13
// 调研拄点：scratch `2.5.15-细节调研-对话面.md` §2.3「前缀稳定性纪律」
//
// 三条可机检不变量（本节把它们写成代码，✗ 不停留在注释）：
//
//	① 渲染序固定 tools → system → messages；system 层内 stable → context → volatile
//	② 稳定层逐字节不动：易变层（记忆 / 技能 / 时间）只**追加在尾** ⇒ 稳定前缀字节不变
//	③ 稳定前缀 sha256 可现算（StableSHA256）—— 压缩 ✗ 不许动稳定层
//
// ★ 为什么这条最值钱：prompt 缓存 ＝ **前缀逐字节匹配**；前缀里任何一个字节变了，
// 其后的缓存全废。虫族本地卵 / 自架引擎或**无 provider 缓存语义**（设计稿 §四 第 4 项），
// 但「三层 ＋ 缓存边界」的前缀纪律本身可借（省重发成本 ＋ 保确定性 ＝ B13 铁律 4）。
// ─────────────────────────────────────────────────────────────────────────────

// 三大段分节标记（渲染序 tools→system→messages · 机检锚点）。
const (
	MarkerTools    = "<<TOOLS>>"
	MarkerSystem   = "<<SYSTEM>>"
	MarkerMessages = "<<MESSAGES>>"
)

// system 层内三层标记（stable→context→volatile · 机检锚点）。
const (
	MarkerStable   = "<<STABLE>>"
	MarkerContext  = "<<CONTEXT>>"
	MarkerVolatile = "<<VOLATILE>>"
)

// ToolDef —— 一个工具定义（渲染进提示的 tools 段）。★ 工具定义与**顺序**会话内钉死
// （改动工具定义或顺序会作废前缀缓存）。
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Message —— 一条对话消息（渲染进 messages 段）。
//
//	Role — user / assistant / tool / system
//	Kind — message / tool_result / thinking / summary（压缩面靠它识别陈旧块）
type Message struct {
	Role    string `json:"role"`
	Kind    string `json:"kind,omitempty"`
	Content string `json:"content"`
}

// Layers —— 三层对话提示（A5：stable / context / volatile 按缓存前缀序装配）。
//
//	Stable   — 稳定层：虫族身份 ＋ 收尾契约 ＋ 工具定义（按固定顺序）· ★ 一个字节不动
//	Context  — 上下文层：`--ctx` / 工作区快照（按会话变 ⇒ 紧随稳定层）
//	Volatile — 易变层：技能索引 / 记忆 / 时间（★ 只**追加在尾**，✗ 不重排、✗ 不插到前面）
type Layers struct {
	Stable   string   `json:"stable"`
	Context  string   `json:"context,omitempty"`
	Volatile []string `json:"volatile,omitempty"`
}

// Prompt —— 一次模型请求的装配面（tools ＋ system 三层 ＋ messages）。
type Prompt struct {
	Tools    []ToolDef `json:"tools,omitempty"`
	System   Layers    `json:"system"`
	Messages []Message `json:"messages,omitempty"`
}

// writeTools —— 渲染 tools 段（确定性：逐条按给定顺序 · ✗ 不排序）。
func writeTools(b *bytes.Buffer, ts []ToolDef) {
	b.WriteString(MarkerTools)
	b.WriteByte('\n')
	for _, t := range ts {
		b.WriteString(t.Name)
		if t.Description != "" {
			b.WriteByte('\t')
			b.WriteString(t.Description)
		}
		b.WriteByte('\n')
	}
}

// StablePrefix —— 稳定前缀字节：tools ＋ system 的 stable ＋ context（缓存可命中的稳定段）。
//
// ★ 渲染序里 volatile 与 messages **都在它之后** ⇒ 稳定前缀 = Render 的**前若干字节**，
// 故「易变层只追加在尾 ⇒ 稳定前缀逐字节不变」是**构造保证**（不变量②）。
func (p Prompt) StablePrefix() []byte {
	var b bytes.Buffer
	writeTools(&b, p.Tools)
	b.WriteString(MarkerSystem)
	b.WriteByte('\n')
	b.WriteString(MarkerStable)
	b.WriteByte('\n')
	b.WriteString(p.System.Stable)
	b.WriteByte('\n')
	b.WriteString(MarkerContext)
	b.WriteByte('\n')
	b.WriteString(p.System.Context)
	b.WriteByte('\n')
	return b.Bytes()
}

// StableSHA256 —— 稳定前缀的 sha256（十六进制）—— 不变量②/③ 的现算读数。
func (p Prompt) StableSHA256() string {
	sum := sha256.Sum256(p.StablePrefix())
	return hex.EncodeToString(sum[:])
}

// Render —— 按固定序渲染整份提示：tools → system(stable→context→volatile) → messages。
// ★ 返回值**以 StablePrefix() 为前缀**（不变量②的构造依据）。
func (p Prompt) Render() []byte {
	var buf bytes.Buffer
	buf.Write(p.StablePrefix()) // 稳定前缀逐字节打头
	buf.WriteString(MarkerVolatile)
	buf.WriteByte('\n')
	for _, v := range p.System.Volatile {
		buf.WriteString(v)
		buf.WriteByte('\n')
	}
	buf.WriteString(MarkerMessages)
	buf.WriteByte('\n')
	for _, m := range p.Messages {
		buf.WriteString(m.Role)
		buf.WriteByte('\t')
		buf.WriteString(m.Kind)
		buf.WriteByte('\t')
		buf.WriteString(m.Content)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// RenderOrderOK —— 机检渲染序不变量①：tools < system < messages 且 stable < context < volatile。
func (p Prompt) RenderOrderOK() bool {
	r := p.Render()
	iTools := bytes.Index(r, []byte(MarkerTools))
	iSys := bytes.Index(r, []byte(MarkerSystem))
	iMsg := bytes.Index(r, []byte(MarkerMessages))
	iStable := bytes.Index(r, []byte(MarkerStable))
	iCtx := bytes.Index(r, []byte(MarkerContext))
	iVol := bytes.Index(r, []byte(MarkerVolatile))
	if iTools < 0 || iSys < 0 || iMsg < 0 || iStable < 0 || iCtx < 0 || iVol < 0 {
		return false
	}
	return iTools < iSys && iSys < iMsg && // 三大段序
		iStable < iCtx && iCtx < iVol && iVol < iMsg // system 内三层序
}

// AppendVolatile —— 往易变层**尾部追加**条目（记忆 / 技能 / 时间）。
//
// ★ 只追加：既有条目**顺序与字节不变** ⇒ 稳定前缀逐字节不变（不变量②的落地形态）。
// 返回新值（✗ 不改原值的底层数组，避免调用方共享切片被意外改写）。
func (p Prompt) AppendVolatile(items ...string) Prompt {
	out := p
	out.System.Volatile = append(append([]string(nil), p.System.Volatile...), items...)
	return out
}
