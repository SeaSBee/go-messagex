#!/bin/bash

# Run Benchmarks Script for go-messagex
# This script runs comprehensive benchmarks and generates reports

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BENCHMARK_TIME=${BENCHMARK_TIME:-30s}
BENCHMARK_MEMORY=${BENCHMARK_MEMORY:-false}
BENCHMARK_CPU=${BENCHMARK_CPU:-false}
OUTPUT_DIR=${OUTPUT_DIR:-./benchmark_results}
DATE=$(date +%Y%m%d_%H%M%S)

echo -e "${BLUE}🚀 Starting go-messagex Benchmarks${NC}"
echo -e "${BLUE}============================================${NC}"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Function to run benchmarks
run_benchmark() {
    local test_pattern="$1"
    local output_file="$2"
    local description="$3"
    
    echo -e "${YELLOW}📊 Running $description...${NC}"
    
    # Base benchmark command
    local cmd="go test -bench=$test_pattern -benchtime=$BENCHMARK_TIME -timeout=10m"
    
    # Add memory profiling if enabled
    if [ "$BENCHMARK_MEMORY" = "true" ]; then
        cmd="$cmd -benchmem -memprofile=${output_file}.mem"
    fi
    
    # Add CPU profiling if enabled
    if [ "$BENCHMARK_CPU" = "true" ]; then
        cmd="$cmd -cpuprofile=${output_file}.cpu"
    fi
    
    # Run benchmark and save output
    echo "Running: $cmd"
    eval "$cmd" ./tests/unit ./tests/benchmarks 2>&1 | tee "${output_file}.log"
    
    echo -e "${GREEN}✅ Completed $description${NC}"
    echo ""
}

# Function to generate summary report
generate_summary() {
    local summary_file="$OUTPUT_DIR/benchmark_summary_$DATE.md"
    
    echo -e "${YELLOW}📋 Generating benchmark summary...${NC}"
    
    cat > "$summary_file" << EOF
# Benchmark Summary Report

**Generated:** $(date)
**Duration:** $BENCHMARK_TIME per benchmark
**Memory Profiling:** $BENCHMARK_MEMORY
**CPU Profiling:** $BENCHMARK_CPU

## System Information
- **OS:** $(uname -s)
- **Architecture:** $(uname -m)
- **Go Version:** $(go version)
- **CPU Cores:** $(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo "Unknown")
- **Memory:** $(free -h 2>/dev/null | grep Mem | awk '{print $2}' || echo "Unknown")

## Benchmark Results

EOF

    # Add results from each benchmark log
    for log_file in "$OUTPUT_DIR"/*.log; do
        if [ -f "$log_file" ]; then
            benchmark_name=$(basename "$log_file" .log)
            echo "### $benchmark_name" >> "$summary_file"
            echo "" >> "$summary_file"
            echo '```' >> "$summary_file"
            
            # Extract benchmark results (lines containing Benchmark and ns/op)
            grep -E "^Benchmark.*\s+[0-9]+\s+[0-9]+(\.[0-9]+)?\s+ns/op" "$log_file" >> "$summary_file" 2>/dev/null || true
            
            echo '```' >> "$summary_file"
            echo "" >> "$summary_file"
        fi
    done
    
    echo -e "${GREEN}✅ Summary report generated: $summary_file${NC}"
}

# Main benchmark execution
echo -e "${BLUE}Configuration:${NC}"
echo "  - Benchmark Time: $BENCHMARK_TIME"
echo "  - Memory Profiling: $BENCHMARK_MEMORY"
echo "  - CPU Profiling: $BENCHMARK_CPU"
echo "  - Output Directory: $OUTPUT_DIR"
echo ""

# Run unit benchmarks
run_benchmark "BenchmarkMessage" \
    "$OUTPUT_DIR/message_benchmark_$DATE" \
    "Message Operations"

run_benchmark "BenchmarkMockPublisher" \
    "$OUTPUT_DIR/publisher_benchmark_$DATE" \
    "Publisher Performance"

run_benchmark "BenchmarkMockConsumer" \
    "$OUTPUT_DIR/consumer_benchmark_$DATE" \
    "Consumer Performance"

run_benchmark "BenchmarkLatencyHistogram" \
    "$OUTPUT_DIR/latency_benchmark_$DATE" \
    "Latency Histogram"

run_benchmark "BenchmarkPerformanceMonitor" \
    "$OUTPUT_DIR/performance_monitor_benchmark_$DATE" \
    "Performance Monitor"

run_benchmark "BenchmarkValidator" \
    "$OUTPUT_DIR/validator_benchmark_$DATE" \
    "Validation Performance"

run_benchmark "BenchmarkCodec" \
    "$OUTPUT_DIR/codec_benchmark_$DATE" \
    "Codec Performance"

run_benchmark "BenchmarkObservability" \
    "$OUTPUT_DIR/observability_benchmark_$DATE" \
    "Observability Performance"

run_benchmark "BenchmarkErrorHandling" \
    "$OUTPUT_DIR/error_handling_benchmark_$DATE" \
    "Error Handling Performance"

run_benchmark "BenchmarkHealthManager" \
    "$OUTPUT_DIR/health_manager_benchmark_$DATE" \
    "Health Manager Performance"

run_benchmark "BenchmarkTestMessageFactory" \
    "$OUTPUT_DIR/test_factory_benchmark_$DATE" \
    "Test Message Factory"

# Run comprehensive benchmarks (if directory exists)
if [ -d "./tests/benchmarks" ]; then
    echo -e "${YELLOW}🔥 Running Comprehensive Benchmarks...${NC}"
    
    run_benchmark "BenchmarkComprehensivePublisher" \
        "$OUTPUT_DIR/comprehensive_publisher_$DATE" \
        "Comprehensive Publisher Scenarios"
    
    run_benchmark "BenchmarkComprehensiveConsumer" \
        "$OUTPUT_DIR/comprehensive_consumer_$DATE" \
        "Comprehensive Consumer Scenarios"
    
    run_benchmark "BenchmarkEndToEnd" \
        "$OUTPUT_DIR/end_to_end_$DATE" \
        "End-to-End Scenarios"
    
    run_benchmark "BenchmarkHighThroughput" \
        "$OUTPUT_DIR/high_throughput_$DATE" \
        "High Throughput Scenario"
    
    run_benchmark "BenchmarkLowLatency" \
        "$OUTPUT_DIR/low_latency_$DATE" \
        "Low Latency Scenario"
    
    run_benchmark "BenchmarkMemoryEfficiency" \
        "$OUTPUT_DIR/memory_efficiency_$DATE" \
        "Memory Efficiency Scenario"
    
    run_benchmark "BenchmarkScalability" \
        "$OUTPUT_DIR/scalability_$DATE" \
        "Scalability Testing"
    
    run_benchmark "BenchmarkMessageSizes" \
        "$OUTPUT_DIR/message_sizes_$DATE" \
        "Message Size Impact"
fi

# Generate summary report
generate_summary

# Optional: Run pprof analysis if profiles were generated
if [ "$BENCHMARK_CPU" = "true" ] || [ "$BENCHMARK_MEMORY" = "true" ]; then
    echo -e "${YELLOW}📈 Profile files generated for analysis:${NC}"
    ls -la "$OUTPUT_DIR"/*.{cpu,mem} 2>/dev/null || true
    echo ""
    echo -e "${BLUE}To analyze profiles, use:${NC}"
    echo "  go tool pprof <profile_file>"
    echo "  go tool pprof -http=:8080 <profile_file>"
fi

echo -e "${GREEN}🎉 All benchmarks completed successfully!${NC}"
echo -e "${GREEN}Results saved to: $OUTPUT_DIR${NC}"
echo ""
echo -e "${BLUE}Quick performance check:${NC}"
echo "  cat $OUTPUT_DIR/benchmark_summary_$DATE.md"
