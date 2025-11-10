# vLLM API Test Script

## Overview
The `test_vllm.php` script tests all OpenAI-compatible endpoints available on your vLLM server.

## Prerequisites
- PHP installed with cURL extension enabled
- vLLM server running on `http://localhost:1234`
- Model loaded and ready

## Usage

### Basic Usage
```bash
cd vllm
php test_vllm.php
```

### Configuration
Edit the configuration variables at the top of `test_vllm.php`:

```php
$VLLM_BASE_URL = 'http://localhost:1234';  // Your vLLM server URL
$MODEL_NAME = 'llama-3.1-8b';              // Your served model name
$API_KEY = null;                            // Set if API key is enabled
```

## Tests Included

1. **Health Check** - Verifies server is running
2. **List Models** - Retrieves available models
3. **Chat Completion** - Standard chat completion (non-streaming)
4. **Chat Completion (Streaming)** - Streaming chat responses
5. **Completion** - Legacy completion endpoint
6. **Function Calling** - Tests function calling capabilities
7. **System Messages** - Tests system message handling

## Expected Output

The script will:
- Display colored output for each test
- Show success/failure status
- Display response content where applicable
- Provide a summary at the end

## Troubleshooting

### "Connection refused" error
- Ensure vLLM container is running: `docker ps`
- Check if port 1234 is correct in the script

### "Model not found" error
- Verify the model name matches your `--served-model-name` in docker-compose.yml
- Check vLLM logs: `docker logs vllm`

### cURL not enabled
- Install PHP cURL extension
- On Windows: Uncomment `extension=curl` in php.ini
- On Linux: `sudo apt-get install php-curl`

## Example Output

```
============================================================
vLLM OpenAI-Compatible API Test Suite
============================================================
Server: http://localhost:1234
Model: llama-3.1-8b
API Key: Not set (open access)

=== Test 1: Health Check ===
✓ Health check passed

=== Test 2: List Models ===
✓ Models retrieved successfully
Available models:
  - llama-3.1-8b

...
```

## Manual Testing with cURL

You can also test manually using cURL:

```bash
# List models
curl http://localhost:1234/v1/models

# Chat completion
curl http://localhost:1234/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3.1-8b",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }'
```

