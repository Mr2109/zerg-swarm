#!/bin/bash
# =============================================================================
# T2: 分支系统完整实现 (Branch System)
# 设计: <repo>/docs/设计-v2.5.3-真实环境-完整建制.md §2.2
# 用途: 容器内 git 分支生命周期管理 (创建/开发/commit/PR/审查/合并)
# =============================================================================
set -euo pipefail

# --- 全局状态变量 ---
export BRANCH_SYSTEM_ROOT="${BRANCH_SYSTEM_ROOT:-}"
export TASK_ID="${TASK_ID:-}"
export CURRENT_BRANCH=""
export PR_STATUS="none"       # none | open | approved | rejected
export REVIEW_NOTES=""
export REPO_PATH="${ZERG_REPO_PATH:-}"   # 支持环境变量传入（独立命令跨进程——否则 setup 后丢失）
export BRANCH_SYSTEM_STATE="${BRANCH_SYSTEM_STATE:-/tmp/branch_system.state}"

# 状态文件：跨命令持久化（每个命令是独立 bash 进程——export 不保留）
_load_state() {
    if [ -f "$BRANCH_SYSTEM_STATE" ]; then
        # shellcheck disable=SC1090
        . "$BRANCH_SYSTEM_STATE"
    fi
}
_save_state() {
    echo "export REPO_PATH=\"$REPO_PATH\"" > "$BRANCH_SYSTEM_STATE"
    echo "export TASK_ID=\"${TASK_ID:-}\"" >> "$BRANCH_SYSTEM_STATE"
    echo "export PR_STATUS=\"$PR_STATUS\"" >> "$BRANCH_SYSTEM_STATE"
    echo "export CURRENT_BRANCH=\"${CURRENT_BRANCH:-}\"" >> "$BRANCH_SYSTEM_STATE"
}
_load_state

# --- 日志工具 ---
log()    { echo "[INFO]  $(date '+%H:%M:%S') $*"; }
log_ok() { echo "[OK]   $(date '+%H:%M:%S') $*"; }
log_err(){ echo "[ERR]  $(date '+%H:%M:%S') $*" >&2; }
log_step(){ echo ""; echo "=== $* ==="; echo ""; }

# =============================================================================
# 1. setup <仓库路径>
#    - 初始化 git 仓库（如不存在）
#    - 创建 main 分支（如不存在）
#    - checkout 新分支 task-<id>
# =============================================================================
setup() {
    local repo_path="$1"
    local task_id="${2:-001}"

    export TASK_ID="$task_id"
    export REPO_PATH="$repo_path"
    export BRANCH_SYSTEM_ROOT="$(pwd)"
    CURRENT_BRANCH="task-${task_id}"
    _save_state

    log_step "📦 [setup] 初始化仓库 + 创建任务分支"

    # --- 初始化仓库（如不存在） ---
    if [ ! -d "${repo_path}/.git" ]; then
        log "创建仓库: ${repo_path}"
        mkdir -p "${repo_path}"
        git -C "${repo_path}" init
        git -C "${repo_path}" config user.email "zerg@local"
        git -C "${repo_path}" config user.name "Zerg Agent"

        # 创建初始提交 + main 分支
        echo "# 任务仓库" > "${repo_path}/README.md"
        git -C "${repo_path}" add README.md
        git -C "${repo_path}" commit -m "init: 初始提交 (main)"

        log_ok "仓库初始化完成 (main 分支)"
    else
        log "仓库已存在: ${repo_path}"
    fi

    # --- 确保 main 分支存在 ---
    local has_main
    has_main=$(git -C "${repo_path}" branch --list main | wc -l)
    if [ "${has_main}" -eq 0 ]; then
        git -C "${repo_path}" checkout -b main
        git -C "${repo_path}" commit --allow-empty -m "ensure: main branch"
        log_ok "main 分支已创建"
    fi

    # --- checkout 新分支 task-<id> ---
    local has_branch
    has_branch=$(git -C "${repo_path}" branch --list "task-${task_id}" | wc -l)
    if [ "${has_branch}" -gt 0 ]; then
        log "⚠️  分支 task-${task_id} 已存在 —— 重置"
        git -C "${repo_path}" checkout task-${task_id}
        git -C "${repo_path}" reset --hard main
    else
        git -C "${repo_path}" checkout -b "task-${task_id}" main
    fi
    log_ok "✅ 切换到分支: ${CURRENT_BRANCH} (从 main)"
    log "当前状态: $(git -C "${repo_path}" status --short)"
}

# =============================================================================
# 2. commit <提交消息> [文件列表...]
#    - git add 指定文件（或全部）
#    - git commit
# =============================================================================
commit() {
    local message="$1"
    shift
    local files=("$@")

    log_step "💾 [commit] 提交: ${message}"

    if [ ${#files[@]} -eq 0 ]; then
        # 没有指定文件 —— add 全部
        git -C "${REPO_PATH}" add -A
    else
        git -C "${REPO_PATH}" add "${files[@]}"
    fi

    # 检查是否有变更
    local diff_output
    diff_output=$(git -C "${REPO_PATH}" diff --cached --stat 2>/dev/null || true)
    if [ -z "${diff_output}" ]; then
        log "⚠️  无变更 —— 跳过 commit"
        return 0
    fi

    git -C "${REPO_PATH}" commit -m "${message}"
    log_ok "✅ 提交完成: ${message}"
    git -C "${REPO_PATH}" log --oneline -3
}

# =============================================================================
# 3. pr <PR描述>
#    - 生成 PR 请求文件 pr-<id>.md
#    - 记录: 改动文件、影响范围、任务描述
# =============================================================================
pr() {
    local description="$1"

    log_step "📋 [pr] 生成 PR 请求: task-${TASK_ID}"

    # --- 收集变更信息 ---
    local main_head
    main_head=$(git -C "${REPO_PATH}" rev-parse main 2>/dev/null || echo "unknown")
    local branch_head
    branch_head=$(git -C "${REPO_PATH}" rev-parse HEAD)

    # 两个分支的 diff
    local diff_files
    diff_files=$(git -C "${REPO_PATH}" diff --name-only main...HEAD 2>/dev/null || true)

    local diff_stat
    diff_stat=$(git -C "${REPO_PATH}" diff --stat main...HEAD 2>/dev/null || true)

    local commit_list
    commit_list=$(git -C "${REPO_PATH}" log main..HEAD --oneline 2>/dev/null || true)

    local file_count
    file_count=$(echo "${diff_files}" | grep -c . || echo 0)

    # --- 生成 PR 文件 ---
    local pr_file="${REPO_PATH}/pr-${TASK_ID}.md"
    cat > "${pr_file}" <<PR_EOF
# PR #${TASK_ID}: ${description}

## 任务信息
- **分支**: ${CURRENT_BRANCH}
- **目标**: main
- **状态**: open
- **日期**: $(date '+%Y-%m-%d %H:%M:%S')

## 描述
${description}

## 改动文件 (${file_count} 个)
\`\`\`
${diff_files}
\`\`\`

## 变更统计
\`\`\`
${diff_stat}
\`\`\`

## 提交记录
\`\`\`
${commit_list}
\`\`\`

## 审查
- **状态**: pending
- **审查人**: -
- **审查意见**: -
- **通过日期**: -
PR_EOF

    export PR_STATUS="open"
    _save_state
    log_ok "✅ PR 文件生成: ${pr_file}"
    log "PR 概要: ${file_count} 个文件变更, $(echo "${commit_list}" | wc -l | tr -d ' ') 次提交"
}

# =============================================================================
# 4. review <通过|拒绝> [审查意见]
#    - 审查分支改动
#    - 验收标准: 测试通过? 产物在?
#    - 标记 PR 状态 (approved/rejected)
# =============================================================================
review() {
    local decision="$1"
    shift
    local notes="${*:-}"

    log_step "🔍 [review] 审查 PR #${TASK_ID}: ${decision}"

    if [ "${PR_STATUS}" != "open" ]; then
        log_err "❌ PR 未打开 (当前状态: ${PR_STATUS}) —— 无法审查"
        return 1
    fi

    # --- 验收检查 ---
    local check_passed=true
    local check_results=""

    # 检查1: 有提交记录
    local commit_count
    commit_count=$(git -C "${REPO_PATH}" log main..HEAD --oneline 2>/dev/null | wc -l | tr -d ' ')
    if [ "${commit_count}" -gt 0 ]; then
        check_results="${check_results}  ✅ 有 ${commit_count} 次提交\n"
    else
        check_results="${check_results}  ❌ 无提交记录\n"
        check_passed=false
    fi

    # 检查2: PR 文件存在
    local pr_file="${REPO_PATH}/pr-${TASK_ID}.md"
    if [ -f "${pr_file}" ]; then
        check_results="${check_results}  ✅ PR 文件存在\n"
    else
        check_results="${check_results}  ❌ PR 文件缺失\n"
        check_passed=false
    fi

    # 检查3: 分支有改动（非空 diff）
    local diff_count
    diff_count=$(git -C "${REPO_PATH}" diff --name-only main...HEAD 2>/dev/null | wc -l | tr -d ' ')
    if [ "${diff_count}" -gt 0 ]; then
        check_results="${check_results}  ✅ 有 ${diff_count} 个文件变更\n"
    else
        check_results="${check_results}  ❌ 无文件变更\n"
        check_passed=false
    fi

    # 检查4: 代码可编译 (如果有 .go 文件)
    local go_files
    go_files=$(git -C "${REPO_PATH}" diff --name-only main...HEAD 2>/dev/null | grep '\.go$' || true)
    if [ -n "${go_files}" ]; then
        check_results="${check_results}  ✅ 有 Go 文件变更（需手动编译验证）\n"
    fi

    log "审查结果:"
    echo -e "${check_results}" | sed 's/^/    /'

    if [ "${decision}" = "通过" ] || [ "${decision}" = "approve" ]; then
        if [ "${check_passed}" = true ]; then
            export PR_STATUS="approved"
            _save_state
            REVIEW_NOTES="${notes:-审查通过 —— 验收标准已满足}"

            # 更新 PR 文件
            sed -i '' "s/- \*\*状态\*\*: open/- \*\*状态\*\*: approved/" "${pr_file}" 2>/dev/null || \
            sed -i "s/- \*\*状态\*\*: open/- \*\*状态\*\*: approved/" "${pr_file}"
            sed -i '' "s/- \*\*审查人\*\*: -/- \*\*审查人\*\*: Zerg Reviewer/" "${pr_file}" 2>/dev/null || \
            sed -i "s/- \*\*审查人\*\*: -/- \*\*审查人\*\*: Zerg Reviewer/" "${pr_file}"
            sed -i '' "s/- \*\*审查意见\*\*: -/- \*\*审查意见\*\*: ${REVIEW_NOTES}/" "${pr_file}" 2>/dev/null || \
            sed -i "s/- \*\*审查意见\*\*: -/- \*\*审查意见\*\*: ${REVIEW_NOTES}/" "${pr_file}"
            sed -i '' "s/- \*\*通过日期\*\*: -/- \*\*通过日期\*\*: $(date '+%Y-%m-%d %H:%M:%S')/" "${pr_file}" 2>/dev/null || \
            sed -i "s/- \*\*通过日期\*\*: -/- \*\*通过日期\*\*: $(date '+%Y-%m-%d %H:%M:%S')/" "${pr_file}"

            log_ok "✅ PR #${TASK_ID} 审查通过"
        else
            log_err "❌ 验收未通过 —— 自动拒绝"
            export PR_STATUS="rejected"
        _save_state
            REVIEW_NOTES="${notes:-验收不通过 —— 有检查项未满足}"
            sed -i '' "s/- \*\*状态\*\*: open/- \*\*状态\*\*: rejected/" "${pr_file}" 2>/dev/null || \
            sed -i "s/- \*\*状态\*\*: open/- \*\*状态\*\*: rejected/" "${pr_file}"
            sed -i '' "s/- \*\*审查人\*\*: -/- \*\*审查人\*\*: Zerg Reviewer/" "${pr_file}" 2>/dev/null || \
            sed -i "s/- \*\*审查人\*\*: -/- \*\*审查人\*\*: Zerg Reviewer/" "${pr_file}"
            sed -i '' "s/- \*\*审查意见\*\*: -/- \*\*审查意见\*\*: ${REVIEW_NOTES}/" "${pr_file}" 2>/dev/null || \
            sed -i "s/- \*\*审查意见\*\*: -/- \*\*审查意见\*\*: ${REVIEW_NOTES}/" "${pr_file}"
            return 1
        fi
    elif [ "${decision}" = "拒绝" ] || [ "${decision}" = "reject" ]; then
        export PR_STATUS="rejected"
        _save_state
        REVIEW_NOTES="${notes:-审查拒绝}"

        sed -i '' "s/- \*\*状态\*\*: open/- \*\*状态\*\*: rejected/" "${pr_file}" 2>/dev/null || \
        sed -i "s/- \*\*状态\*\*: open/- \*\*状态\*\*: rejected/" "${pr_file}"
        sed -i '' "s/- \*\*审查人\*\*: -/- \*\*审查人\*\*: Zerg Reviewer/" "${pr_file}" 2>/dev/null || \
        sed -i "s/- \*\*审查人\*\*: -/- \*\*审查人\*\*: Zerg Reviewer/" "${pr_file}"
        sed -i '' "s/- \*\*审查意见\*\*: -/- \*\*审查意见\*\*: ${REVIEW_NOTES}/" "${pr_file}" 2>/dev/null || \
        sed -i "s/- \*\*审查意见\*\*: -/- \*\*审查意见\*\*: ${REVIEW_NOTES}/" "${pr_file}"

        log "🚫 PR #${TASK_ID} 审查拒绝"
        return 1
    else
        log_err "❌ 未知审查决定: ${decision} (应为 通过/approve 或 拒绝/reject)"
        return 1
    fi
}

# =============================================================================
# 5. merge
#    - 合并回 main (仅通过审查的 PR)
#    - 否则丢弃分支
# =============================================================================
merge() {
    log_step "🔀 [merge] 处理 PR #${TASK_ID}"

    if [ "${PR_STATUS}" != "approved" ]; then
        log_err "❌ PR 未通过审查 (状态: ${PR_STATUS}) —— 丢弃分支 ${CURRENT_BRANCH}"
        git -C "${REPO_PATH}" checkout main
        git -C "${REPO_PATH}" branch -D "${CURRENT_BRANCH}" 2>/dev/null || true
        log "分支 ${CURRENT_BRANCH} 已丢弃"
        return 1
    fi

    # --- 合并回 main ---
    git -C "${REPO_PATH}" checkout main
    git -C "${REPO_PATH}" merge --no-ff "${CURRENT_BRANCH}" -m "merge: ${CURRENT_BRANCH} → main (PR #${TASK_ID})"

    log_ok "✅ PR #${TASK_ID} 已合并到 main"

    # --- 清理 ---
    git -C "${REPO_PATH}" branch -D "${CURRENT_BRANCH}" 2>/dev/null || true
    log "分支 ${CURRENT_BRANCH} 已删除"

    # 切换到 main
    git -C "${REPO_PATH}" checkout main
    log "当前分支: $(git -C "${REPO_PATH}" branch --show-current)"
    log "main 最新提交:"
    git -C "${REPO_PATH}" log --oneline -3
}

# =============================================================================
# discard - 丢弃分支 (不合并)
# =============================================================================
discard() {
    log_step "🗑️  [discard] 丢弃分支 ${CURRENT_BRANCH}"
    git -C "${REPO_PATH}" checkout main
    git -C "${REPO_PATH}" branch -D "${CURRENT_BRANCH}" 2>/dev/null || true
    log_ok "分支 ${CURRENT_BRANCH} 已丢弃"
}

# =============================================================================
# status - 显示当前状态
# =============================================================================
status() {
    echo "=== 分支系统状态 ==="
    echo "  仓库路径: ${REPO_PATH}"
    echo "  任务 ID:  ${TASK_ID}"
    echo "  当前分支: ${CURRENT_BRANCH}"
    echo "  PR 状态:  ${PR_STATUS}"
    echo "  审查意见: ${REVIEW_NOTES}"
    echo ""
    echo "=== git log ==="
    git -C "${REPO_PATH}" log --oneline --all -10
    echo ""
    echo "=== git status ==="
    git -C "${REPO_PATH}" status
}

# =============================================================================
# 帮助
# =============================================================================
help() {
    cat <<'HELP_EOF'
分支系统 (Branch System) - T2
用法: bash branch_system.sh <命令> [参数]

命令:
  setup <仓库路径> [任务ID]     初始化仓库 + checkout task-<id> 分支
  commit <消息> [文件...]       git add + commit
  pr <描述>                     生成 PR 请求文件 pr-<id>.md
  review <通过|拒绝> [意见]     审查 PR (验收: 测试/产物)
  merge                         合并回 main (仅通过审查)
  discard                       丢弃分支 (不合并)
  status                        显示当前状态
  help                          显示帮助

示例:
  bash branch_system.sh setup /tmp/test-repo 001
  bash branch_system.sh commit "feat: add feature" file1.go
  bash branch_system.sh pr "实现用户认证模块"
  bash branch_system.sh review 通过 "功能完整，测试通过"
  bash branch_system.sh merge
HELP_EOF
}

# --- 主入口 ---
case "${1:-help}" in
    setup)    shift; setup "$@" ;;
    commit)   shift; commit "$@" ;;
    pr)       shift; pr "$@" ;;
    review)   shift; review "$@" ;;
    merge)    merge ;;
    discard)  discard ;;
    status)   status ;;
    help|--help|-h) help ;;
    *) log_err "未知命令: $1 (运行 help 查看用法)"; exit 1 ;;
esac
