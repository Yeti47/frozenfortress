#!/bin/sh
set -e

# Start the Ollama server in the background.
ollama serve &
SERVE_PID=$!

# Wait until the server accepts requests.
until ollama list >/dev/null 2>&1; do
    sleep 2
done

# Pull the configured model so it is ready before any OCR requests arrive.
# Subsequent starts are fast because Ollama skips layers already in the volume.
# A pull failure is logged as a warning but does NOT stop the server — the
# container should still run so other services can start.
if [ -n "$OLLAMA_MODEL" ]; then
    echo "Pulling Ollama model: $OLLAMA_MODEL"
    if ! ollama pull "$OLLAMA_MODEL"; then
        echo "WARNING: failed to pull model '$OLLAMA_MODEL'. OCR will be unavailable until the model is present." >&2
    fi
fi

# Hand off to the server process.
wait $SERVE_PID
