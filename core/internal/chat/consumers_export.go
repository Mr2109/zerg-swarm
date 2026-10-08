package chat

// consumers_export.go —— 「版本目录定格」读数的**同源导出面**（账 `GAP-20260927-129` ④）。
//
// 为什么单开一件：只读命令 `zerg doc consumers` 与判据 `check-doc-consumers` 要**现读**
// `zerg_overview` 解析出的版本目录，而 `currentVersionDir()` / `releaseArtifactGlob` 在包内是
// **小写未导出** ⇒ 命令面（package main）够不着。本件只做**薄导出**：直接转调既有函数与常量，
// ✗ 不新写第二套解析（真源仍是 `chat_tool_overview.go` 的 `currentVersionDir()` 与 `releaseArtifactGlob`）。
//
// ✗ 不改 `currentVersionDir()` / `latestVersionDir()` / `hasReleaseArtifact` 一行行为（本件零副作用）。

// OverviewCurrentVersionDir —— `zerg_overview` 现读消费的版本目录（= currentVersionDir()）。
func OverviewCurrentVersionDir() string { return currentVersionDir() }

// ReleaseArtifactGlob —— 「已发布版」判据 glob（= releaseArtifactGlob：使用指南）；
// 供命令面/判据现读同一份「含发布件」口径（✗ 不在包外另写一个字面量）。
const ReleaseArtifactGlob = releaseArtifactGlob
