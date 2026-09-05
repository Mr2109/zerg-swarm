package api

// review_task.go — 阶段2: 复查模型接入（2026-08-20 设计——任务git全生命周期）
// 执行完成（确定性验证过）→ 自动派复查任务（跨家族模型 B——与执行 A 不同源）
// 复查模型读执行 commit → 按 rubric 评分 → 写复查报告 → 决定通过/打回

import (
	"container/heap"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 复查模型池（跨家族——真独立——防同源偏见）
// 执行用 example-35b-v2 → 复查用 Qwen/gemma（不同训练源/架构）
var reviewModelPool = []string{
	"example-26b-review",     // Google 家族——复查专用别名（只走 local 本机——单槽隔离——Mr210909-05拍板）
	"GLM-4.7-Flash",          // 智谱家族
	"Qwen3.8-27B",            // Qwen 家族
	"example-35b-v2",         // Ornith 家族
	"example-30b",       // Meta 家族（Mr2109新增）
	"Nemotron-3.5-Lightning", // NVIDIA 家族（Mr2109新增）
}

// pickReviewModel 选复查模型（与执行模型不同家族——跨源）
// execModel: 执行模型名——复查不能同源（同家族也不行——Qwen3.8 执行不能 Qwen3.6 复查）
func pickReviewModel(execModel string) string {
	// 家族分类（按训练源/架构——跨家族才算独立）
	familyOf := func(m string) string {
		ml := strings.ToLower(m)
		switch {
		case strings.Contains(ml, "example-35b-v2"):
			return "example-35b-v2"
		case strings.Contains(ml, "qwen"):
			return "qwen"
		case strings.Contains(ml, "gemma"):
			return "gemma"
		case strings.Contains(ml, "glm"):
			return "glm"
		case strings.Contains(ml, "deepseek") || strings.Contains(ml, "ds4"):
			return "deepseek"
		case strings.Contains(ml, "qwable"):
			return "qwable"
		case strings.Contains(ml, "muse") || strings.Contains(ml, "glimmer") || strings.Contains(ml, "llama"):
			return "meta" // Meta 家族（Muse-Glimmer 是 Meta 开源）
		case strings.Contains(ml, "nemotron") || strings.Contains(ml, "nvidia"):
			return "nvidia" // NVIDIA 家族
		default:
			return "unknown"
		}
	}
	execFamily := familyOf(execModel)
	// 从池里选第一个不同家族的
	for _, m := range reviewModelPool {
		if familyOf(m) != execFamily {
			return m
		}
	}
	// 全同源（极端）——用池里第一个（记录——但不理想）
	return reviewModelPool[0]
}

// 复查任务完成 → 决策（通过/打回——Mr2109: 成功与否由复查模型决定）
// 调用方持 s.mu.Lock
func (s *MasterScheduler) handleReviewDoneLocked(reviewTask *Task) {
	// 找执行任务
	execTask, ok := s.history[reviewTask.RefTaskID]
	if !ok {
		log.Printf("⚠️ 复查 %s 找不到执行任务 %s——复查结果丢弃", reviewTask.ID, reviewTask.RefTaskID)
		return
	}
	// 读复查结论（复查报告——2026-09-05 治本: 缺失=打回 不默认通过——
	// 原逻辑: 复查模型没产出报告→pass——复查模型失败=执行任务自动成功（假完成绿通道——实测 internal-health-check 案例走通）
	// 现逻辑: 报告缺失/过短/无结论 → rework（打回重做）——复查是闸门不是橡皮章
	conclusion := "rework" // 默认打回（复查没给出明确通过证据=不通过）
	reportPath := FindTaskReport(reviewTask.Workdir)
	// 补认 review-report.md（复查任务约定文件名——原只读 internal-task-report.md——读写路径错位）
	if reportPath == "" {
		for _, cand := range []string{
			filepath.Join(filepath.Dir(reviewTask.Workdir), "review-report.md"),
			filepath.Join(reviewTask.Workdir, "review-report.md"),
		} {
			if _, err := os.Stat(cand); err == nil {
				reportPath = cand
				break
			}
		}
	}
	if reportPath != "" {
		if content, err := os.ReadFile(reportPath); err == nil {
			low := strings.ToLower(string(content))
			// 报告过短（<100 字）= 复查模型敷衍——打回（防 26 字空报告蒙混——实测案例）
			if len(content) < 100 {
				log.Printf("🔁 总调度: 复查 %s 报告过短（%d 字节——疑似敷衍）——打回", reviewTask.ID, len(content))
			} else if strings.Contains(low, "打回") || strings.Contains(low, "不通过") || strings.Contains(low, "rework") {
				// 打回关键词（去掉裸 "fail"——报告里提 "测试未 fail" 等反述误判）
				conclusion = "rework"
			} else if strings.Contains(low, "通过") || strings.Contains(low, "pass") {
				// 明确含通过语义词才 pass
				conclusion = "pass"
			}
		}
	}
	// S7 复查误伤治理: 复查自身失败（无报告=复查模型没产出结论——复查环节故障）
	// ≠ 执行任务不通过。锅不能让执行任务背——重派复查（换模型池）——上限 2 次，超限才打回
	if conclusion == "rework" && reportPath == "" {
		log.Printf("🔁 总调度: 复查 %s 无报告——复查自身失败（非执行不通过）——处理见下", reviewTask.ID)
	}
	if conclusion == "rework" && reportPath == "" {
		// 复查失败重派（换模型池——非打回执行任务）
		execTask.ReviewFailedCount++
		if execTask.ReviewFailedCount <= 2 {
			reviewRetryID := fmt.Sprintf("review-retry-%s-%d", sanitizeID(execTask.ID), execTask.ReviewFailedCount)
			// 从复查池选一个与上次不同的模型
			nextModel := pickAlternateReviewModel(reviewTask.Model, execTask.Model)
			retryTask := &Task{
				ID:          reviewRetryID,
				Description: fmt.Sprintf("重派复查任务（第 %d 次——上次复查模型未产出报告——非执行任务问题）。\n被复查任务: %s\n执行报告: %s\n要求: 独立审查执行产物与报告——写结论到 review-report.md（结论必须含「通过」或「打回」——附理由）。", execTask.ReviewFailedCount, execTask.ID, FindTaskReport(execTask.Workdir)),
				Priority:    PriorityExternal - 1,
				Type:        "review",
				Model:       nextModel,
				Workdir:     reviewTask.Workdir,
				Status:      "queued",
				RefTaskID:   execTask.ID,
				RefWorktree: execTask.RefWorktree,
				ReplanCount: execTask.ReplanCount,
				ReviewCount: execTask.ReviewCount,
				CreatedAt:   time.Now(),
			}
			heap.Push(&s.queue, retryTask)
			log.Printf("🔁 总调度: 复查 %s 自身失败——重派复查 %s（模型 %s——第 %d/2 次）", reviewTask.ID, reviewRetryID, nextModel, execTask.ReviewFailedCount)
			return
		}
		// 复查重派超限——按原逻辑打回（但注记是复查系统失败）
		log.Printf("⚠️ 总调度: 复查重派 %d 次仍失败——回退打回执行任务 %s（复查系统故障注记）", execTask.ReviewFailedCount, execTask.ID)
	}
	if conclusion == "rework" {
		// v2.5.5 阶段3（设计-20260820）: Replan 循环——打回 → 派重做任务（同模型 A——带复查意见）
		// 上限: 执行-复查 3 次 + Replan 总 5 次（VeriMAP 默认——防死循环）——超限标 failed
		execTask.ReviewCount++
		if execTask.ReviewCount > 5 || execTask.ReplanCount >= 3 {
			execTask.Status = "failed"
			execTask.FailReason = fmt.Sprintf("复查打回 %d 次超限（重做 %d 次）: %s", execTask.ReviewCount, execTask.ReplanCount, shortReason(reportPath))
			log.Printf("🔁 总调度: 复查 %s 打回任务 %s 超限（复查%d次/重做%d次）——标 failed", reviewTask.ID, execTask.ID, execTask.ReviewCount, execTask.ReplanCount)
		} else {
			// 派重做任务（同模型 A——带复查意见——S6: 续作语义非全量重做）
			execTask.ReplanCount++
			reworkID := "rework-" + sanitizeID(execTask.ID) + "-" + fmt.Sprintf("%d", execTask.ReplanCount)
			// S6 断点数据 env（结晶任务打回——CA 侧 LoadPlan/LoadCrystals 恢复）
			extraEnv := []string{
				"ZERG_TASK_DIR=" + taskDirOf(execTask),
				"ZERG_REVIEW_NOTE=" + shortReason(reportPath),
			}
			reworkTask := &Task{
				ID:          reworkID,
				Description: fmt.Sprintf("续作任务（第 %d 次——复查打回后续作——被复查任务: %s）。\n复查意见（必须按此修正——只修问题不重做已完成部分）: %s\n原任务: %s\n要求: 按复查意见修正——已完成的产物不要重做——写报告到 internal-task-report.md——贴输出。", execTask.ReplanCount, execTask.ID, shortReason(reportPath), execTask.Description),
				Priority:    PriorityExternal - 1,
				Type:        "rework",
				Model:       execTask.Model, // 同执行模型 A
				Workdir:     execTask.Workdir,
				Status:      "queued",
				RefTaskID:   execTask.ID,
				RefWorktree: execTask.RefWorktree,
				ReplanCount: execTask.ReplanCount,
				ReviewCount: execTask.ReviewCount,
				ExtraEnv:    extraEnv,   // S6: 断点恢复+意见注入
				CreatedAt:   time.Now(), // 修复: rework 也设创建时间（UI 执行时长）
			}
			heap.Push(&s.queue, reworkTask)
			log.Printf("🔁 总调度: 复查 %s 打回任务 %s——派重做（第 %d 次——模型 %s）", reviewTask.ID, execTask.ID, execTask.ReplanCount, execTask.Model)
		}
	} else {
		// 通过 → merge 执行任务 worktree（如果还没 merge）
		log.Printf("✅ 总调度: 复查 %s 通过任务 %s（执行模型 %s——复查模型 %s）", reviewTask.ID, execTask.ID, execTask.Model, reviewTask.Model)
		// v2.5.6 修复（2026-08-29 Mr2109——zerg 流程复查）: 执行任务状态 reviewing → done（复查通过才算完成）
		if execTask.Status == "reviewing" {
			execTask.Status = "done"
			log.Printf("✅ 总调度: 任务 %s 复查通过——正式完成（done）", execTask.ID)
		}
		if execTask.RefWorktree != "" {
			if err := mergeWorktree(execTask.RefWorktree, "task-"+sanitizeID(execTask.ID)); err != nil {
				log.Printf("⚠️ 总调度: 任务 %s worktree merge 失败: %v", execTask.ID, err)
			} else {
				log.Printf("✅ 总调度: 任务 %s worktree merge 回 main——任务 git 归档完成", execTask.ID)
				// v2.5.5 修复（2026-08-21 Mr2109发现——UI 报告缺失）: merge 后复制报告到任务目录
				// worktree 删除后报告丢——详情读不到——复制保留（报告不丢）
				copyReportToTaskDir(execTask.ID, execTask.RefWorktree)
			}
		}
	}
	// 落盘（执行任务状态更新）
	saveTasksLocked(s.queue, s.running, s.history)
}

// shortReason 从复查报告提取短原因（前 200 字）
func shortReason(reportPath string) string {
	if reportPath == "" {
		return "复查报告未找到"
	}
	content, err := os.ReadFile(reportPath)
	if err != nil {
		return "复查报告读取失败"
	}
	s := string(content)
	if len(s) > 200 {
		s = s[:200]
	}
	return strings.ReplaceAll(s, "\n", " ")
}

// 复查任务派发（执行完成验证过——自动派复查——跨家族模型 B）
// 注意: 调用方持 s.mu.Lock（在 runTask 的 defer unlock 内）
func (s *MasterScheduler) submitReviewTaskLocked(execTask *Task, reportPath, worktreeDir string) {
	reviewModel := pickReviewModel(execTask.Model)
	// 复查任务（Type=review——优先级低于外部高于内部——复查不该被内部任务挤掉）
	reviewID := "review-" + sanitizeID(execTask.ID)
	// 防止重复派复查（同执行任务只复查一次）
	for _, r := range s.running {
		if r.ID == reviewID {
			return
		}
	}
	reviewTask := &Task{
		ID:          reviewID,
		Description: buildReviewPrompt(execTask, reportPath) + fmt.Sprintf("\n\n【复查报告要求（硬性）】: 复查报告写到执行任务的任务目录: /tmp/zerg-tasks/%s/review-report.md（绝对路径——用 write 工具写这个路径——复查所有内容都在这个单独文档——不写共享路径——硬性要求）", sanitizeID(execTask.ID)),
		Priority:    PriorityExternal - 1, // 外部之下——内部之上
		Type:        "review",
		Model:       reviewModel,
		Workdir:     execTask.Workdir,
		Status:      "queued",
		// v2.5.5 修复（2026-08-21 Mr2109发现）: 复查任务执行时间不对——CreatedAt 零值
		// 直接 heap.Push 不走 Submit——手动设创建时间（UI 执行时长用）
		CreatedAt: time.Now(),
		// 关联执行任务（复查完成后决策用）
	}
	// 记录关联（复查任务 → 执行任务）
	reviewTask.RefTaskID = execTask.ID
	reviewTask.RefWorktree = worktreeDir
	// 执行任务也记 worktree（复查通过后 merge 用）
	execTask.RefWorktree = worktreeDir
	heap.Push(&s.queue, reviewTask)
	log.Printf("🔍 总调度: 任务 %s 派复查（模型 %s——跨家族——复查 %s）", reviewID, reviewModel, execTask.ID)
}

// buildReviewPrompt 构造复查任务描述（复查模型读执行结果——rubric 评分）
// execTask: 执行任务（含描述/报告路径/worktree）
func buildReviewPrompt(execTask *Task, reportPath string) string {
	return fmt.Sprintf(`复查任务（执行/复查双模型——你是复查者——决定成败）。

被复查任务: %s
执行模型: %s
报告路径: %s

要求:
1. 读执行任务的报告（报告路径）——验证内容真实性
2. 检查执行结果（git diff/产物——如果有 worktree 分支——看改动）
3. 按以下 rubric 评分（每维度: 通过/不通过）:
   - 真实性: 报告描述是否匹配实际改动（防谎报/幻觉）
   - 完成度: 任务要求是否全部做到
   - 质量: 逻辑/风格/健壮性（代码任务看代码质量）
   - 验证: 是否真的跑过测试/验证（有证据）
4. 结论（必须明确）: 通过 或 打回
   - 通过: 说明理由
   - 打回: 列出具体问题 + 改进建议（给执行模型重做用）
5. 写复查报告到 internal-task-report.md（复查报告——追加或覆盖——含 rubric 评分表 + 结论）
6. 贴输出（评分表 + 结论）

注意: 你是独立复查者——不受执行模型影响——只看证据（报告+代码+产物）——真复查真评分。`, execTask.Description, execTask.Model, reportPath)
}

// taskDirOf — 任务目录（S6 断点数据位置——/tmp/zerg-tasks/<ID>/）
func taskDirOf(t *Task) string {
	return filepath.Join("/tmp/zerg-tasks", t.ID)
}

// pickAlternateReviewModel — S7: 复查重派选模型（与上次不同优先——池小则接受同款）
func pickAlternateReviewModel(lastReviewModel, execModel string) string {
	next := pickReviewModel(execModel)
	if next != lastReviewModel {
		return next
	}
	// 池小无备选——同款重试（换 seed 意义靠重跑本身）
	log.Printf("⚠️ 复查池无备选模型——同款 %s 重派（重跑换随机性）", next)
	return next
}
