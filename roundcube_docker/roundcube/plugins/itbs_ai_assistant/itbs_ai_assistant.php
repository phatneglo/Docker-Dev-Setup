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
        $this->register_action('plugin.itbs_ai_context', [$this, 'handle_context']);

        $rcmail = rcmail::get_instance();
        $this->api->output->set_env('itbs_ai_enabled', true);
        $this->api->output->set_env('itbs_ai_title', $rcmail->config->get('itbs_ai_title', 'PNP Mail AI'));
    }

    public function handle_ai()
    {
        $rcmail = rcmail::get_instance();
        $allowed = ['compose', 'reply', 'summarize', 'translate', 'phishing', 'ask', 'chat'];

        $action = rcube_utils::get_input_value('_ai_action', rcube_utils::INPUT_POST);
        $text = rcube_utils::get_input_value('_text', rcube_utils::INPUT_POST, true);
        $prompt = rcube_utils::get_input_value('_prompt', rcube_utils::INPUT_POST, true);
        $tone = rcube_utils::get_input_value('_tone', rcube_utils::INPUT_POST, true);
        $language = rcube_utils::get_input_value('_language', rcube_utils::INPUT_POST, true);
        $subject = rcube_utils::get_input_value('_subject', rcube_utils::INPUT_POST, true);
        $context = rcube_utils::get_input_value('_context', rcube_utils::INPUT_POST, true);
        $stream = rcube_utils::get_input_value('_stream', rcube_utils::INPUT_POST) === '1';

        if (!in_array($action, $allowed, true)) {
            $this->json_response(['error' => 'Invalid AI action.'], 400);
        }

        $baseUrl = rtrim($rcmail->config->get('itbs_ai_api_url', 'http://ai-api:8090'), '/');
        $secret = $rcmail->config->get('itbs_ai_shared_secret', '');
        $url = $baseUrl . '/v1/ai/' . $action . ($stream ? '/stream' : '');

        $payload_data = [
            'action' => $action,
            'text' => $text,
            'prompt' => $prompt,
            'tone' => $tone,
            'language' => $language,
            'subject' => $subject,
        ];

        if ($context !== '') {
            $decoded_context = json_decode($context, true);
            if (is_array($decoded_context)) {
                $payload_data['context'] = $decoded_context;
            }
        }

        $payload = json_encode($payload_data);

        if (!function_exists('curl_init')) {
            $this->json_response(['error' => 'PHP cURL extension is required for the AI plugin.'], 500);
        }
        $headers = ['Content-Type: application/json'];
        if ($secret !== '') {
            $headers[] = 'X-ITBS-AI-SECRET: ' . $secret;
        }

        if ($stream) {
            $this->stream_ai_response($url, $headers, $payload);
        }

        $ch = curl_init($url);

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

    public function handle_context()
    {
        $rcmail = rcmail::get_instance();
        $storage = $rcmail->get_storage();
        $folder = rcube_utils::get_input_value('_folder', rcube_utils::INPUT_POST, true);
        $folder = $folder !== '' ? $folder : $storage->get_folder();
        $folder = $folder !== '' ? $folder : 'INBOX';

        $context = [
            'folder' => $folder,
            'user' => $rcmail->user ? $rcmail->user->get_username() : '',
            'unread_count' => null,
            'recent_messages' => [],
        ];

        try {
            $storage->set_folder($folder);
            $context['unread_count'] = (int) $storage->count($folder, 'UNSEEN');
            $messages = $storage->list_messages($folder, 1, 'date', 'DESC', 10);

            foreach ((array) $messages as $message) {
                $context['recent_messages'][] = [
                    'uid' => isset($message->uid) ? (string) $message->uid : '',
                    'subject' => $this->clean_context_text($message->subject ?? ''),
                    'from' => $this->clean_context_text($message->from ?? ''),
                    'to' => $this->clean_context_text($message->to ?? ''),
                    'date' => $this->clean_context_text($message->date ?? ''),
                    'size' => isset($message->size) ? (int) $message->size : null,
                    'flags' => isset($message->flags) && is_array($message->flags) ? array_values($message->flags) : [],
                ];
            }
        } catch (Exception $e) {
            $context['error'] = 'Could not load mailbox context.';
        }

        $this->json_response($context, 200);
    }

    private function stream_ai_response($url, $headers, $payload)
    {
        set_time_limit(0);
        ignore_user_abort(true);

        while (ob_get_level() > 0) {
            ob_end_flush();
        }

        http_response_code(200);
        header('Content-Type: text/plain; charset=utf-8');
        header('Cache-Control: no-cache, no-transform');
        header('X-Accel-Buffering: no');

        $started = false;
        $ch = curl_init($url);
        curl_setopt_array($ch, [
            CURLOPT_POST => true,
            CURLOPT_HTTPHEADER => $headers,
            CURLOPT_POSTFIELDS => $payload,
            CURLOPT_CONNECTTIMEOUT => 10,
            CURLOPT_TIMEOUT => 120,
            CURLOPT_HEADERFUNCTION => function ($ch, $header) {
                if (stripos($header, 'X-ITBS-AI-Mock:') === 0 && !headers_sent()) {
                    header(trim($header));
                }
                return strlen($header);
            },
            CURLOPT_WRITEFUNCTION => function ($ch, $chunk) use (&$started) {
                $started = true;
                echo $chunk;
                flush();
                return strlen($chunk);
            },
        ]);

        $ok = curl_exec($ch);
        $error = curl_error($ch);
        $status = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        if ($ok === false || $status < 200 || $status > 299) {
            $message = $ok === false ? $error : 'AI API stream failed with status ' . $status;
            echo ($started ? "\n\n" : '') . '[AI stream error: ' . $message . ']';
            flush();
        }

        exit;
    }

    private function json_response($payload, $status = 200)
    {
        http_response_code($status);
        header('Content-Type: application/json; charset=utf-8');
        echo json_encode($payload);
        exit;
    }

    private function clean_context_text($value)
    {
        $value = is_string($value) ? $value : (string) $value;
        $value = rcube_mime::decode_mime_string($value);
        $value = html_entity_decode(strip_tags($value), ENT_QUOTES | ENT_HTML5, 'UTF-8');
        return trim(preg_replace('/\s+/', ' ', $value));
    }
}
