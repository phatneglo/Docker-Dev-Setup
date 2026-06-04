<?php
/**
 * ITBS AI Assistant for Roundcube.
 * Adds AI compose/reply/summarize/translate/phishing actions through a Go AI API.
 */
class itbs_ai_assistant extends rcube_plugin
{
    public $task = 'mail';

    public function init()
    {
        $this->load_config();
        $this->include_stylesheet('itbs_ai_assistant.css');
        $this->include_script('itbs_ai_assistant.js');
        $this->register_action('plugin.itbs_ai', [$this, 'handle_ai']);

        $rcmail = rcmail::get_instance();
        $this->api->output->set_env('itbs_ai_enabled', true);
        $this->api->output->set_env('itbs_ai_title', $rcmail->config->get('itbs_ai_title', 'PNP Mail AI'));
    }

    public function handle_ai()
    {
        $rcmail = rcmail::get_instance();
        $allowed = ['compose', 'reply', 'summarize', 'translate', 'phishing', 'ask'];

        $action = rcube_utils::get_input_value('_ai_action', rcube_utils::INPUT_POST);
        $text = rcube_utils::get_input_value('_text', rcube_utils::INPUT_POST, true);
        $prompt = rcube_utils::get_input_value('_prompt', rcube_utils::INPUT_POST, true);
        $tone = rcube_utils::get_input_value('_tone', rcube_utils::INPUT_POST, true);
        $language = rcube_utils::get_input_value('_language', rcube_utils::INPUT_POST, true);
        $subject = rcube_utils::get_input_value('_subject', rcube_utils::INPUT_POST, true);

        if (!in_array($action, $allowed, true)) {
            $this->json_response(['error' => 'Invalid AI action.'], 400);
        }

        $baseUrl = rtrim($rcmail->config->get('itbs_ai_api_url', 'http://ai-api:8090'), '/');
        $secret = $rcmail->config->get('itbs_ai_shared_secret', '');
        $url = $baseUrl . '/v1/ai/' . $action;

        $payload = json_encode([
            'action' => $action,
            'text' => $text,
            'prompt' => $prompt,
            'tone' => $tone,
            'language' => $language,
            'subject' => $subject,
        ]);

        if (!function_exists('curl_init')) {
            $this->json_response(['error' => 'PHP cURL extension is required for the AI plugin.'], 500);
        }

        $ch = curl_init($url);
        $headers = ['Content-Type: application/json'];
        if ($secret !== '') {
            $headers[] = 'X-ITBS-AI-SECRET: ' . $secret;
        }

        curl_setopt_array($ch, [
            CURLOPT_POST => true,
            CURLOPT_HTTPHEADER => $headers,
            CURLOPT_POSTFIELDS => $payload,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_CONNECTTIMEOUT => 10,
            CURLOPT_TIMEOUT => 80,
        ]);

        $response = curl_exec($ch);
        $error = curl_error($ch);
        $status = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        if ($response === false) {
            $this->json_response(['error' => 'Could not connect to AI API: ' . $error], 502);
        }

        $decoded = json_decode($response, true);
        if ($status < 200 || $status > 299) {
            $message = is_array($decoded) && isset($decoded['error']) ? $decoded['error'] : 'AI API request failed.';
            $this->json_response(['error' => $message, 'status' => $status], $status ?: 502);
        }

        $this->json_response(is_array($decoded) ? $decoded : ['result' => $response], 200);
    }

    private function json_response($payload, $status = 200)
    {
        http_response_code($status);
        header('Content-Type: application/json; charset=utf-8');
        echo json_encode($payload);
        exit;
    }
}
