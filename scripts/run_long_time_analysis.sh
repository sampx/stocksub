#!/bin/bash

# 长时间运行时间字段分析测试脚本
# 用于持续收集不同时间段的时间字段格式数据

echo "🕐 长时间运行时间字段分析测试"
echo "========================================"
echo ""
echo "此脚本将持续运行数小时，收集不同时间段的股票时间字段格式数据"
echo ""

# 默认配置
RUN_DURATION=${RUN_DURATION:-"5m"}      # 运行时长
COLLECT_INTERVAL=${COLLECT_INTERVAL:-"10s"}  # 收集间隔

# 计算超时时间（运行时长 + 30分钟缓冲）
calculate_timeout() {
    local duration=$1
    case $duration in
        *h) 
            hours=${duration%h}
            echo "$((hours + 1))h"
            ;;
        *m)
            minutes=${duration%m}
            echo "$((minutes + 30))m"
            ;;
        *s)
            seconds=${duration%s}
            echo "$((seconds + 1800))s"
            ;;
        *)
            echo "5h"  # 默认超时
            ;;
    esac
}

TEST_TIMEOUT=$(calculate_timeout "$RUN_DURATION")

echo "📋 运行配置:"
echo "  运行时长: $RUN_DURATION"
echo "  收集间隔: $COLLECT_INTERVAL"
echo "  数据目录: tests/data/long_run"
echo ""

# 环境变量说明
echo "💡 可配置参数:"
echo "  RUN_DURATION=2h      # 设置运行时长"
echo "  COLLECT_INTERVAL=5m  # 设置收集间隔"
echo "  TEST_FORCE_CACHE=1   # 强制使用缓存模式"
echo ""

# 安全提醒
echo "⚠️  安全提醒:"
echo "  1. 长时间运行可能进行多次API调用"
echo "  2. 建议在网络环境良好时运行"
echo "  3. 测试期间请不要关闭终端"
echo "  4. 数据将保存在 tests/data/long_run 目录"
echo ""

# 确认运行
read -p "是否开始长时间运行测试？(y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ 已取消运行"
    exit 0
fi

# 检查go环境
if ! command -v go &> /dev/null; then
    echo "❌ 错误：未找到Go环境，请先安装Go"
    exit 1
fi

# 检查项目目录
if [ ! -f "go.mod" ] || [ ! -d "pkg/provider/tencent" ]; then
    echo "❌ 错误：请在stocksub项目根目录下运行此脚本"
    exit 1
fi

# 创建数据目录
mkdir -p tests/data/long_run

echo ""
echo "🚀 开始长时间运行测试..."
echo "📊 预计进行 $(echo "$RUN_DURATION" | sed 's/h/小时/g' | sed 's/m/分钟/g') 的数据收集"
echo ""

# 设置环境变量并运行测试
export LONG_RUN=1
export RUN_DURATION=$RUN_DURATION
export COLLECT_INTERVAL=$COLLECT_INTERVAL

# 运行长时间测试
go test -v -tags=integration ./tests -run TestTimeFieldConsistencyLongRun -timeout "$TEST_TIMEOUT"

echo ""
echo "✅ 长时间运行测试完成！"
echo ""
echo "📁 生成的文件:"
echo "  - tests/data/long_run/collection_*.json (每次收集的详细数据)"
echo "  - tests/data/long_run/summary_report_*.md (汇总分析报告)"
echo ""
echo "📈 后续步骤:"
echo "  1. 查看汇总报告了解发现的时间格式"
echo "  2. 分析不同时间段的格式变化"
echo "  3. 根据结果优化parseTime函数"