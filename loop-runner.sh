#!/bin/bash
# 严格模式:-e 遇错即停,-u 变量未定义即错,-o pipefail 管道错即错
set -euo pipefail

# 配置路径
VAULT_DIR="00_Loop_Memory"
CONTRACT_FILE="$VAULT_DIR/Loop_Contracts.md"
VAULT_FILE="$VAULT_DIR/Loop_Memory_Vault.md"
AUDIT_FILE="$VAULT_DIR/Audit_Log.md"
TREND_LOG="$VAULT_DIR/.loop_trend.tmp"
MAX_LOOPS=8

# 配置运行 Agent CLI (可选: agy, claude, hermes, codex, openclaw)
AGENT_CLI="${AGENT_CLI:-agy}"

# 自动兼容 Windows 下 the agy 路径
export PATH="$PATH:/mnt/c/Users/sj929/AppData/Local/agy/bin:/c/Users/sj929/AppData/Local/agy/bin:$HOME/AppData/Local/agy/bin"

# 【行数双轨制配置】
HARD_THRESHOLD=150      # 单轮硬熔断阈值(防止大范围盲目重写)
SOFT_THRESHOLD=70       # 软趋势阈值(codec 类精准修复的建议改动上限)
MAX_SOFT_STREAKS=2      # 连续 N 轮超过软阈值则视为渐进式坍塌

mkdir -p "$VAULT_DIR"
touch "$CONTRACT_FILE" "$VAULT_FILE" "$AUDIT_FILE"
rm -f "$TREND_LOG" && touch "$TREND_LOG"

find_cli() {
    local cmd=$1
    if command -v "$cmd" &>/dev/null; then
        echo "$cmd"
    elif command -v "${cmd}.exe" &>/dev/null; then
        echo "${cmd}.exe"
    elif command -v "${cmd}.cmd" &>/dev/null; then
        echo "${cmd}.cmd"
    else
        echo "$cmd"
    fi
}

run_agent() {
    local prompt_content=$1
    case "$AGENT_CLI" in
        agy)
            $(find_cli agy) --dangerously-skip-permissions --print "$prompt_content"
            ;;
        claude)
            $(find_cli claude) --dangerously-skip-permissions -p "$prompt_content"
            ;;
        hermes)
            $(find_cli hermes) chat -q "$prompt_content"
            ;;
        codex)
            $(find_cli codex) exec "$prompt_content"
            ;;
        openclaw)
            $(find_cli openclaw) session send "$prompt_content"
            ;;
        *)
            echo "❌ Unsupported AGENT_CLI: $AGENT_CLI" >&2
            exit 1
            ;;
    esac
}

loop_count=0

safe_sed() {
    local pattern=$1
    local file=$2
    if [[ "${OSTYPE:-}" == "darwin"* ]]; then
        sed -i.bak "$pattern" "$file" && rm "${file}.bak"
    else
        sed -i "$pattern" "$file"
    fi
}

# 检查是否连续 N 轮超过软阈值(从最近一轮往前数,中间一旦有一轮回落就中断连续性)
check_soft_streaks() {
    local threshold=$1
    local max_streaks=$2
    local streak=0

    if [ -s "$TREND_LOG" ]; then
        while read -r line; do
            [ -z "$line" ] && continue
            if [ "$line" -gt "$threshold" ]; then
                streak=$((streak + 1))
                if [ "$streak" -ge "$max_streaks" ]; then
                    return 0  # 触发渐进式失控
                fi
            else
                break  # 一旦有一轮回落,连续性即被打断
            fi
        done < <(tac "$TREND_LOG" 2>/dev/null || tail -r "$TREND_LOG" 2>/dev/null || awk '{a[NR]=$0} END {for(i=NR;i>0;i--) print a[i]}' "$TREND_LOG")
    fi
    return 1  # 未触发
}

while [ "$loop_count" -lt "$MAX_LOOPS" ]; do
    # 关键修正:((loop_count++)) 在 loop_count 从 0 增至 1 时,
    # 后置自增表达式的值是自增前的 0,会被 bash 算术命令判定为
    # "失败"(退出码 1),在独立语句位置(不在 if/while 条件里)会
    # 被 set -e 直接终止脚本。改用赋值形式规避这个经典陷阱。
    loop_count=$((loop_count + 1))

    echo "======================================"
    echo "🔄 Loop Iteration: $loop_count / $MAX_LOOPS"
    echo "======================================"

    # ----------------------------------------------------
    # STEP 1: 运行执行 Agent (Execution Agent)
    # ----------------------------------------------------
    echo "🚀 Launching Execution Agent..."
    EXECUTION_PROMPT=$(cat prompts/execution_agent.md | sed "s/{{env.CURRENT_LOOP_COUNT}}/$loop_count/g")
    run_agent "$EXECUTION_PROMPT"

    if ! grep -qF "[STATUS: READY FOR AUDIT]" "$VAULT_FILE"; then
        echo "❌ Execution Agent halted or requested manual intervention."
        exit 1
    fi

    safe_sed 's/\[STATUS: READY FOR AUDIT\]//g' "$VAULT_FILE"

    # ----------------------------------------------------
    # STEP 2: 构建"密室"沙箱 + 多维代码突变审计 (Mutation Guard)
    # ----------------------------------------------------
    echo "🛡️ Preparing sterile sandbox for auditor..."
    SANDBOX_DIR=".audit_sandbox"
    rm -rf "$SANDBOX_DIR" && mkdir -p "$SANDBOX_DIR"

    git diff > "$SANDBOX_DIR/current_changes.diff"

    total_changes=$(git diff --numstat | awk '{add+=$1; del+=$2} END {print add+del+0}')
    total_changes=${total_changes:-0}
    echo "$total_changes" >> "$TREND_LOG"
    echo "📊 Current iteration code mutation: $total_changes lines."

    # 1) 单轮硬红线
    if [ "$total_changes" -gt "$HARD_THRESHOLD" ]; then
        echo "🚨 HARD MUTATION BLOWOUT TRIGGERED!"
        echo "Execution Agent modified $total_changes lines, exceeding hard limit ($HARD_THRESHOLD)."
        safe_sed 's/^- \*\*Status\*\*:.*/- **Status**: Blocked (Hard Mutation Blowout)/' "$CONTRACT_FILE"
        safe_sed "s/^- \*\*Gatekeeper Status\*\*:.*/- **Gatekeeper Status**: Halted (${total_changes} lines > hard limit)/" "$CONTRACT_FILE"
        rm -rf "$SANDBOX_DIR" "$TREND_LOG"
        exit 3
    fi

    # 2) 连续软红线(渐进式坍塌)
    if check_soft_streaks "$SOFT_THRESHOLD" "$MAX_SOFT_STREAKS"; then
        echo "🚨 PROGRESSIVE COLLAPSE DETECTED!"
        echo "Execution Agent has $MAX_SOFT_STREAKS+ consecutive rounds modifying > $SOFT_THRESHOLD lines."
        safe_sed 's/^- \*\*Status\*\*:.*/- **Status**: Blocked (Progressive Collapse)/' "$CONTRACT_FILE"
        safe_sed 's/^- \*\*Gatekeeper Status\*\*:.*/- **Gatekeeper Status**: Halted (progressive collapse)/' "$CONTRACT_FILE"
        rm -rf "$SANDBOX_DIR" "$TREND_LOG"
        exit 4
    fi

    modified_files=$(git diff --name-only)
    for file in $modified_files; do
        if [ -f "$file" ]; then
            mkdir -p "$SANDBOX_DIR/$(dirname "$file")"
            cp "$file" "$SANDBOX_DIR/$file"
        fi
    done

    # ----------------------------------------------------
    # STEP 3: 计算 Audit ID 并调用审计 Agent
    # ----------------------------------------------------
    raw_last_id=$(grep -oE "Audit ID: #[0-9]+" "$AUDIT_FILE" | awk -F '#' '{print $2}' | sort -n | tail -n1 || true)
    last_id=${raw_last_id:-0}
    next_id=$((last_id + 1))
    timestamp=$(date "+%Y-%m-%d %H:%M:%S")

    echo "🛡️ Launching stateless auditor (Audit ID: #$next_id)..."

    cd "$SANDBOX_DIR"
    AUDIT_PROMPT=$(cat ../prompts/audit_agent.md | sed "s/{{env.AUDIT_TIMESTAMP}}/$timestamp/g" | sed "s/{{env.AUDIT_ID_VAR}}/#$next_id/g")
    FULL_AUDIT_PROMPT="$AUDIT_PROMPT

以下是当前变更内容 (current_changes.diff):
$(cat current_changes.diff)"
    run_agent "$FULL_AUDIT_PROMPT" > audit_result.tmp

    cd ..
    THIS_ROUND_VERDICT="$SANDBOX_DIR/audit_result.tmp"
    cat "$THIS_ROUND_VERDICT" >> "$AUDIT_FILE"

    # ----------------------------------------------------
    # STEP 4: 基于本轮最新结果决策(只看本轮文件,不看历史累积)
    # ----------------------------------------------------
    # 只锁定方括号判定词本身,不依赖 "Verdict:" 前缀的精确格式
    # (审计模板输出的是 Markdown 加粗 "**Verdict**:",与纯文本 "Verdict:" 不是同一个子串)
    if grep -qF "[REJECTED]" "$THIS_ROUND_VERDICT"; then
        echo "🔴 AUDIT REJECTED! Looping back..."
        safe_sed 's/^- \*\*Gatekeeper Status\*\*:.*/- **Gatekeeper Status**: Test: Passed | Audit: Failed/' "$CONTRACT_FILE"
        rm -rf "$SANDBOX_DIR"
        continue
    fi

    if grep -qF "[APPROVED]" "$THIS_ROUND_VERDICT"; then
        echo "🟢 AUDIT APPROVED! Code conforms to engineering specifications."
        safe_sed 's/^- \*\*Status\*\*:.*/- **Status**: Done/' "$CONTRACT_FILE"
        safe_sed 's/^- \*\*Gatekeeper Status\*\*:.*/- **Gatekeeper Status**: Test: Passed | Audit: Passed/' "$CONTRACT_FILE"
        rm -rf "$SANDBOX_DIR" "$TREND_LOG"
        echo "🎉 Task completed successfully in $loop_count iterations!"
        exit 0
    fi

    echo "⚠️  Auditor output did not match expected verdict format. Halting for review."
    echo "    See: $THIS_ROUND_VERDICT"
    rm -rf "$SANDBOX_DIR" "$TREND_LOG"
    exit 5
done

echo "🚨 CRITICAL: Loop budget exhausted ($MAX_LOOPS iterations)."
safe_sed 's/^- \*\*Status\*\*:.*/- **Status**: Blocked (Loop Limit Reached)/' "$CONTRACT_FILE"
rm -f "$TREND_LOG"
exit 2
