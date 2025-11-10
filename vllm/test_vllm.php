<?php
/**
 * vLLM OpenAI-Compatible API Test Script
 * Tests all available OpenAI endpoints on vLLM server
 * 
 * Usage: php test_vllm.php
 */

// Configuration
$VLLM_BASE_URL = 'http://localhost:1234';
$MODEL_NAME = 'llama-3.1-8b'; // Change if your served model name is different
$API_KEY = null; // Set to your API key if authentication is enabled

// Colors for terminal output
class Colors {
    const RESET = "\033[0m";
    const RED = "\033[31m";
    const GREEN = "\033[32m";
    const YELLOW = "\033[33m";
    const BLUE = "\033[34m";
    const MAGENTA = "\033[35m";
    const CYAN = "\033[36m";
    const BOLD = "\033[1m";
}

// Helper function to make API requests
function makeRequest($url, $method = 'GET', $data = null, $stream = false) {
    $ch = curl_init();
    
    curl_setopt($ch, CURLOPT_URL, $url);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_TIMEOUT, 60);
    
    $headers = ['Content-Type: application/json'];
    
    // Add API key if provided
    if ($GLOBALS['API_KEY']) {
        $headers[] = 'Authorization: Bearer ' . $GLOBALS['API_KEY'];
    }
    
    curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
    
    if ($method === 'POST') {
        curl_setopt($ch, CURLOPT_POST, true);
        if ($data) {
            curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($data));
        }
    }
    
    if ($stream) {
        curl_setopt($ch, CURLOPT_WRITEFUNCTION, function($ch, $data) {
            echo $data;
            return strlen($data);
        });
    }
    
    $response = curl_exec($ch);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $error = curl_error($ch);
    
    curl_close($ch);
    
    if ($error) {
        return ['error' => $error, 'http_code' => $httpCode];
    }
    
    return [
        'data' => $stream ? null : json_decode($response, true),
        'raw' => $response,
        'http_code' => $httpCode
    ];
}

// Test functions
function testHealthCheck() {
    echo Colors::CYAN . Colors::BOLD . "\n=== Test 1: Health Check ===\n" . Colors::RESET;
    
    $result = makeRequest($GLOBALS['VLLM_BASE_URL'] . '/health');
    
    if ($result['http_code'] === 200) {
        echo Colors::GREEN . "✓ Health check passed\n" . Colors::RESET;
        return true;
    } else {
        echo Colors::RED . "✗ Health check failed (HTTP {$result['http_code']})\n" . Colors::RESET;
        return false;
    }
}

function testListModels() {
    echo Colors::CYAN . Colors::BOLD . "\n=== Test 2: List Models ===\n" . Colors::RESET;
    
    $result = makeRequest($GLOBALS['VLLM_BASE_URL'] . '/v1/models');
    
    if ($result['http_code'] === 200 && isset($result['data']['data'])) {
        echo Colors::GREEN . "✓ Models retrieved successfully\n" . Colors::RESET;
        echo Colors::YELLOW . "Available models:\n" . Colors::RESET;
        foreach ($result['data']['data'] as $model) {
            echo "  - " . $model['id'] . "\n";
        }
        return true;
    } else {
        echo Colors::RED . "✗ Failed to list models\n" . Colors::RESET;
        if (isset($result['data'])) {
            print_r($result['data']);
        }
        return false;
    }
}

function testChatCompletion() {
    echo Colors::CYAN . Colors::BOLD . "\n=== Test 3: Chat Completion (Non-Streaming) ===\n" . Colors::RESET;
    
    $data = [
        'model' => $GLOBALS['MODEL_NAME'],
        'messages' => [
            [
                'role' => 'system',
                'content' => 'You are a helpful assistant.'
            ],
            [
                'role' => 'user',
                'content' => 'Write a short haiku about programming.'
            ]
        ],
        'temperature' => 0.7,
        'max_tokens' => 100
    ];
    
    $result = makeRequest($GLOBALS['VLLM_BASE_URL'] . '/v1/chat/completions', 'POST', $data);
    
    if ($result['http_code'] === 200 && isset($result['data']['choices'][0])) {
        echo Colors::GREEN . "✓ Chat completion successful\n" . Colors::RESET;
        $message = $result['data']['choices'][0]['message']['content'];
        echo Colors::YELLOW . "Response:\n" . Colors::RESET;
        echo wordwrap($message, 80) . "\n";
        echo Colors::BLUE . "\nUsage: " . json_encode($result['data']['usage'], JSON_PRETTY_PRINT) . "\n" . Colors::RESET;
        return true;
    } else {
        echo Colors::RED . "✗ Chat completion failed\n" . Colors::RESET;
        if (isset($result['data'])) {
            print_r($result['data']);
        }
        return false;
    }
}

function testChatCompletionStreaming() {
    echo Colors::CYAN . Colors::BOLD . "\n=== Test 4: Chat Completion (Streaming) ===\n" . Colors::RESET;
    
    $data = [
        'model' => $GLOBALS['MODEL_NAME'],
        'messages' => [
            [
                'role' => 'user',
                'content' => 'Count from 1 to 5, one number per line.'
            ]
        ],
        'temperature' => 0.7,
        'max_tokens' => 50,
        'stream' => true
    ];
    
    echo Colors::YELLOW . "Streaming response:\n" . Colors::RESET;
    echo Colors::GREEN;
    
    $ch = curl_init();
    curl_setopt($ch, CURLOPT_URL, $GLOBALS['VLLM_BASE_URL'] . '/v1/chat/completions');
    curl_setopt($ch, CURLOPT_POST, true);
    curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($data));
    curl_setopt($ch, CURLOPT_HTTPHEADER, ['Content-Type: application/json']);
    curl_setopt($ch, CURLOPT_WRITEFUNCTION, function($ch, $data) {
        $lines = explode("\n", $data);
        foreach ($lines as $line) {
            if (strpos($line, 'data: ') === 0) {
                $json = substr($line, 6);
                if ($json === '[DONE]') {
                    return strlen($data);
                }
                $decoded = json_decode($json, true);
                if (isset($decoded['choices'][0]['delta']['content'])) {
                    echo $decoded['choices'][0]['delta']['content'];
                }
            }
        }
        return strlen($data);
    });
    
    curl_exec($ch);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);
    
    echo Colors::RESET . "\n";
    
    if ($httpCode === 200) {
        echo Colors::GREEN . "✓ Streaming completion successful\n" . Colors::RESET;
        return true;
    } else {
        echo Colors::RED . "✗ Streaming completion failed (HTTP $httpCode)\n" . Colors::RESET;
        return false;
    }
}

function testCompletion() {
    echo Colors::CYAN . Colors::BOLD . "\n=== Test 5: Completion (Legacy Endpoint) ===\n" . Colors::RESET;
    
    $data = [
        'model' => $GLOBALS['MODEL_NAME'],
        'prompt' => 'The capital of France is',
        'temperature' => 0.7,
        'max_tokens' => 20,
        'stop' => ['.', '\n']
    ];
    
    $result = makeRequest($GLOBALS['VLLM_BASE_URL'] . '/v1/completions', 'POST', $data);
    
    if ($result['http_code'] === 200 && isset($result['data']['choices'][0])) {
        echo Colors::GREEN . "✓ Completion successful\n" . Colors::RESET;
        $text = $result['data']['choices'][0]['text'];
        echo Colors::YELLOW . "Response: " . Colors::RESET . trim($text) . "\n";
        return true;
    } else {
        echo Colors::RED . "✗ Completion failed\n" . Colors::RESET;
        if (isset($result['data'])) {
            print_r($result['data']);
        }
        return false;
    }
}

function testChatWithFunctionCalling() {
    echo Colors::CYAN . Colors::BOLD . "\n=== Test 6: Chat with Function Calling ===\n" . Colors::RESET;
    echo Colors::YELLOW . "Note: Function calling may not be supported by this vLLM version\n" . Colors::RESET;
    
    $data = [
        'model' => $GLOBALS['MODEL_NAME'],
        'messages' => [
            [
                'role' => 'user',
                'content' => 'What is the weather like in San Francisco?'
            ]
        ],
        'functions' => [
            [
                'name' => 'get_weather',
                'description' => 'Get the current weather in a given location',
                'parameters' => [
                    'type' => 'object',
                    'properties' => [
                        'location' => [
                            'type' => 'string',
                            'description' => 'The city and state, e.g. San Francisco, CA'
                        ],
                        'unit' => [
                            'type' => 'string',
                            'enum' => ['celsius', 'fahrenheit']
                        ]
                    ],
                    'required' => ['location']
                ]
            ]
        ],
        'function_call' => 'auto',
        'temperature' => 0.7,
        'max_tokens' => 100
    ];
    
    $result = makeRequest($GLOBALS['VLLM_BASE_URL'] . '/v1/chat/completions', 'POST', $data);
    
    if ($result['http_code'] === 200 && isset($result['data']['choices'][0])) {
        $choice = $result['data']['choices'][0];
        if (isset($choice['message']['function_call'])) {
            echo Colors::GREEN . "✓ Function calling supported and working\n" . Colors::RESET;
            echo Colors::YELLOW . "Function call detected:\n" . Colors::RESET;
            echo "  Function: " . $choice['message']['function_call']['name'] . "\n";
            echo "  Arguments: " . $choice['message']['function_call']['arguments'] . "\n";
        } else {
            echo Colors::YELLOW . "⚠ Function calling not supported (fields ignored by server)\n" . Colors::RESET;
            echo Colors::YELLOW . "Response: " . Colors::RESET . $choice['message']['content'] . "\n";
            echo Colors::BLUE . "This is expected - vLLM may ignore function calling fields\n" . Colors::RESET;
        }
        return true; // Still return true as the request succeeded
    } else {
        echo Colors::RED . "✗ Function calling test failed\n" . Colors::RESET;
        if (isset($result['data'])) {
            print_r($result['data']);
        }
        return false;
    }
}

function testChatWithSystemMessage() {
    echo Colors::CYAN . Colors::BOLD . "\n=== Test 7: Chat with System Message ===\n" . Colors::RESET;
    
    $data = [
        'model' => $GLOBALS['MODEL_NAME'],
        'messages' => [
            [
                'role' => 'system',
                'content' => 'You are a professional code reviewer. Provide concise feedback.'
            ],
            [
                'role' => 'user',
                'content' => 'Review this code: function add(a, b) { return a + b; }'
            ]
        ],
        'temperature' => 0.3,
        'max_tokens' => 150
    ];
    
    $result = makeRequest($GLOBALS['VLLM_BASE_URL'] . '/v1/chat/completions', 'POST', $data);
    
    if ($result['http_code'] === 200 && isset($result['data']['choices'][0])) {
        echo Colors::GREEN . "✓ System message test successful\n" . Colors::RESET;
        $message = $result['data']['choices'][0]['message']['content'];
        echo Colors::YELLOW . "Response:\n" . Colors::RESET;
        echo wordwrap($message, 80) . "\n";
        return true;
    } else {
        echo Colors::RED . "✗ System message test failed\n" . Colors::RESET;
        if (isset($result['data'])) {
            print_r($result['data']);
        }
        return false;
    }
}

// Main execution
echo Colors::BOLD . Colors::MAGENTA . "\n" . str_repeat("=", 60) . "\n";
echo "vLLM OpenAI-Compatible API Test Suite\n";
echo str_repeat("=", 60) . "\n" . Colors::RESET;
echo "Server: " . $VLLM_BASE_URL . "\n";
echo "Model: " . $MODEL_NAME . "\n";
echo "API Key: " . ($API_KEY ? "Set" : "Not set (open access)") . "\n";

$results = [];

$results[] = testHealthCheck();
$results[] = testListModels();
$results[] = testChatCompletion();
$results[] = testChatCompletionStreaming();
$results[] = testCompletion();
$results[] = testChatWithFunctionCalling(); // May show warnings, but still works
$results[] = testChatWithSystemMessage();

// Summary
echo Colors::BOLD . Colors::MAGENTA . "\n" . str_repeat("=", 60) . "\n";
echo "Test Summary\n";
echo str_repeat("=", 60) . "\n" . Colors::RESET;

$passed = count(array_filter($results));
$total = count($results);

echo "Passed: " . Colors::GREEN . "$passed/$total" . Colors::RESET . "\n";
echo "Failed: " . Colors::RED . ($total - $passed) . "/$total" . Colors::RESET . "\n";

if ($passed === $total) {
    echo Colors::GREEN . Colors::BOLD . "\n✓ All tests passed!\n" . Colors::RESET;
    exit(0);
} else {
    echo Colors::RED . Colors::BOLD . "\n✗ Some tests failed\n" . Colors::RESET;
    exit(1);
}

