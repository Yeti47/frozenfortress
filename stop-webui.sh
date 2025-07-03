#!/bin/bash

# Stop script for Frozen Fortress ffwebui
# This script terminates all running ffwebui processes

echo "Stopping Frozen Fortress WebUI..."

# Function to find ffwebui processes (excluding this script)
find_webui_processes() {
    # Look for different patterns, but exclude this script
    local all_pids=""
    
    # Check for bin/ffwebui processes (run from project root)
    local bin_pids=$(pgrep -f "bin/ffwebui" 2>/dev/null || true)
    if [ -n "$bin_pids" ]; then
        all_pids="$all_pids $bin_pids"
    fi
    
    # Check for bin/ffwebui-debug processes (debug build from project root)
    local bin_debug_pids=$(pgrep -f "bin/ffwebui-debug" 2>/dev/null || true)
    if [ -n "$bin_debug_pids" ]; then
        all_pids="$all_pids $bin_debug_pids"
    fi
    
    # Check for ./ffwebui processes (run from bin directory)
    local local_pids=$(pgrep -f "./webui" 2>/dev/null || true)
    if [ -n "$local_pids" ]; then
        all_pids="$all_pids $local_pids"
    fi

    # Check for ./ffwebui processes (run from bin directory)
    local local_pids=$(pgrep -f "./ffwebui" 2>/dev/null || true)
    if [ -n "$local_pids" ]; then
        all_pids="$all_pids $local_pids"
    fi
    
    # Check for ./ffwebui-debug processes (debug build from bin directory)
    local local_debug_pids=$(pgrep -f "./ffwebui-debug" 2>/dev/null || true)
    if [ -n "$local_debug_pids" ]; then
        all_pids="$all_pids $local_debug_pids"
    fi
    
    # Check for go-build ffwebui processes (temporary builds)  
    local exe_pids=$(pgrep -f "exe/ffwebui" 2>/dev/null || true)
    if [ -n "$exe_pids" ]; then
        all_pids="$all_pids $exe_pids"
    fi
    
    # Filter out this script's process and any invalid PIDs
    local filtered_pids=""
    for pid in $all_pids; do
        if [ -n "$pid" ] && [ "$pid" != "$$" ] && kill -0 "$pid" 2>/dev/null; then
            # Make sure it's not this script
            if ! ps -p "$pid" -o cmd= 2>/dev/null | grep -q "stop-webui.sh"; then
                filtered_pids="$filtered_pids $pid"
            fi
        fi
    done
    
    # Clean up the list
    echo "$filtered_pids" | tr ' ' '\n' | grep -v '^$' | sort -u | tr '\n' ' ' | sed 's/^ *//;s/ *$//'
}

echo ""
echo "Looking for running ffwebui processes..."

webui_pids=$(find_webui_processes)

if [ -n "$webui_pids" ]; then
    echo "Found ffwebui processes: $webui_pids"
    echo ""
    echo "Attempting graceful shutdown (SIGTERM)..."
    
    for pid in $webui_pids; do
        echo "  Stopping process $pid..."
        kill -TERM "$pid" 2>/dev/null || true
    done
    
    echo "Waiting 5 seconds for graceful shutdown..."
    sleep 5
    
    # Check which processes are still running
    echo ""
    echo "Checking for remaining processes..."
    still_running=""
    for pid in $webui_pids; do
        if kill -0 "$pid" 2>/dev/null; then
            still_running="$still_running $pid"
        fi
    done
    
    if [ -n "$still_running" ]; then
        echo "Some processes are still running. Force killing..."
        
        for pid in $still_running; do
            echo "  Force killing process $pid..."
            kill -KILL "$pid" 2>/dev/null || true
        done
        
        sleep 2
    fi
    
    # Final verification - check if any ffwebui processes are still running
    final_remaining=$(find_webui_processes)
    
    if [ -z "$final_remaining" ]; then
        echo "✓ All Frozen Fortress WebUI processes have been terminated successfully!"
    else
        echo "⚠ Warning: Some ffwebui processes may still be running:"
        for pid in $final_remaining; do
            ps -p "$pid" -o pid,ppid,cmd --no-headers 2>/dev/null || true
        done
        echo ""
        echo "You may need to manually terminate these processes."
    fi
else
    echo "✓ No Frozen Fortress WebUI processes were found running."
fi

echo ""
echo "Stop operation completed."
