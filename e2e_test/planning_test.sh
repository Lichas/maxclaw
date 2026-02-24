#!/bin/bash
# e2e_test/planning_test.sh - Manual testing for planning system

set -e

echo "=== Planning System Manual Test ==="
echo ""
echo "Prerequisites:"
echo "1. Build maxclaw: make build"
echo "2. Start gateway: maxclaw gateway"
echo "3. Open web UI or use CLI"
echo ""

echo "Test 1: Basic Planning"
echo "----------------------"
echo "Input: '下载分析腾讯最近5年财报'"
echo "Expected:"
echo "- Plan created in ~/.maxclaw/workspace/.sessions/{session}/plan.json"
echo "- Progress shown in responses"
echo "- Steps auto-advance"
echo ""

echo "Test 2: Pause and Resume"
echo "-------------------------"
echo "Input: (a task that exceeds 20 iterations)"
echo "Expected:"
echo "- Plan status becomes 'paused'"
echo "- Response shows progress summary"
echo "- Input '继续' resumes from where left off"
echo ""

echo "Test 3: Step Declaration"
echo "-------------------------"
echo "Input: '帮我完成一个复杂任务'"
echo "LLM Output containing: '[Step] 第一步描述'"
echo "Expected:"
echo "- New step added to plan.json"
echo "- Step appears in progress summary"
echo ""

echo "Verification commands:"
echo "cat ~/.maxclaw/workspace/.sessions/desktop_*/plan.json"
echo ""
