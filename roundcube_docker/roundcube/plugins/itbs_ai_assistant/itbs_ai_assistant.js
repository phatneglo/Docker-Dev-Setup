(function () {
  function ready(fn) {
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', fn);
    else fn();
  }

  ready(function () {
    if (!window.rcmail || !rcmail.env || !rcmail.env.itbs_ai_enabled) return;
    if (document.getElementById('itbs-ai-panel')) return;

    const panel = document.createElement('div');
    panel.id = 'itbs-ai-panel';
    panel.innerHTML = `
      <div class="itbs-ai-header">
        <strong>${escapeHtml(rcmail.env.itbs_ai_title || 'PNP Mail AI')}</strong>
        <button type="button" id="itbs-ai-toggle" title="Hide AI panel">−</button>
      </div>
      <div class="itbs-ai-body">
        <textarea id="itbs-ai-prompt" placeholder="Instruction or question. Example: make this formal, reply politely, translate to Tagalog..."></textarea>
        <div class="itbs-ai-grid">
          <button type="button" data-ai="compose">Compose</button>
          <button type="button" data-ai="reply">Reply</button>
          <button type="button" data-ai="summarize">Summarize</button>
          <button type="button" data-ai="translate">Translate</button>
          <button type="button" data-ai="phishing">Check Risk</button>
          <button type="button" data-ai="ask">Ask</button>
        </div>
        <select id="itbs-ai-language">
          <option value="English">English</option>
          <option value="Tagalog">Tagalog</option>
          <option value="Filipino">Filipino</option>
        </select>
        <select id="itbs-ai-tone">
          <option value="professional">Professional</option>
          <option value="friendly">Friendly</option>
          <option value="formal">Formal</option>
          <option value="short and direct">Short and direct</option>
        </select>
        <textarea id="itbs-ai-result" placeholder="AI result will appear here." readonly></textarea>
        <button type="button" id="itbs-ai-insert">Insert result into compose box</button>
        <div id="itbs-ai-status"></div>
      </div>`;

    document.body.appendChild(panel);

    panel.querySelector('#itbs-ai-toggle').addEventListener('click', function () {
      panel.classList.toggle('is-collapsed');
      this.textContent = panel.classList.contains('is-collapsed') ? '+' : '−';
    });

    panel.querySelectorAll('[data-ai]').forEach(function (btn) {
      btn.addEventListener('click', function () {
        callAI(this.getAttribute('data-ai'));
      });
    });

    panel.querySelector('#itbs-ai-insert').addEventListener('click', insertIntoCompose);
  });

  async function callAI(action) {
    const status = document.getElementById('itbs-ai-status');
    const result = document.getElementById('itbs-ai-result');
    const prompt = document.getElementById('itbs-ai-prompt').value || '';
    const language = document.getElementById('itbs-ai-language').value || 'English';
    const tone = document.getElementById('itbs-ai-tone').value || 'professional';

    status.textContent = 'AI is working...';
    result.value = '';

    try {
      const form = new FormData();
      form.append('_token', rcmail.env.request_token || '');
      form.append('_ai_action', action);
      form.append('_text', getCurrentEmailText());
      form.append('_prompt', prompt);
      form.append('_language', language);
      form.append('_tone', tone);
      form.append('_subject', getSubject());

      const response = await fetch(rcmail.url('plugin.itbs_ai'), {
        method: 'POST',
        body: form,
        credentials: 'same-origin'
      });
      const data = await response.json();

      if (!response.ok || data.error) {
        throw new Error(data.error || 'AI request failed.');
      }

      result.value = data.result || JSON.stringify(data, null, 2);
      status.textContent = data.mock ? 'Mock AI result. Add your API key to enable real AI.' : 'Done.';
    } catch (err) {
      status.textContent = 'Error: ' + err.message;
    }
  }

  function getCurrentEmailText() {
    const selected = String(window.getSelection ? window.getSelection() : '').trim();
    if (selected) return selected;

    const compose = document.querySelector('#composebody');
    if (compose && compose.value) return compose.value;

    const htmlEditor = document.querySelector('iframe.tox-edit-area__iframe, iframe#mceu_0_ifr, iframe[id*="composebody"]');
    if (htmlEditor && htmlEditor.contentDocument && htmlEditor.contentDocument.body) {
      return htmlEditor.contentDocument.body.innerText || htmlEditor.contentDocument.body.textContent || '';
    }

    const message = document.querySelector('#messagebody, .message-htmlpart, .message-text, .body, .message-part');
    if (message) return message.innerText || message.textContent || '';

    return '';
  }

  function getSubject() {
    const subjectInput = document.querySelector('input[name="_subject"], #compose-subject');
    if (subjectInput && subjectInput.value) return subjectInput.value;

    const subjectView = document.querySelector('.subject, h2.subject');
    return subjectView ? (subjectView.innerText || subjectView.textContent || '') : '';
  }

  function insertIntoCompose() {
    const value = document.getElementById('itbs-ai-result').value;
    if (!value) return;

    const compose = document.querySelector('#composebody');
    if (compose) {
      compose.value = compose.value ? compose.value + '\n\n' + value : value;
      compose.dispatchEvent(new Event('change', { bubbles: true }));
      return;
    }

    const htmlEditor = document.querySelector('iframe.tox-edit-area__iframe, iframe#mceu_0_ifr, iframe[id*="composebody"]');
    if (htmlEditor && htmlEditor.contentDocument && htmlEditor.contentDocument.body) {
      htmlEditor.contentDocument.body.innerHTML += '<p>' + escapeHtml(value).replace(/\n/g, '<br>') + '</p>';
      return;
    }

    alert('Open the compose window first, then click Insert.');
  }

  function escapeHtml(value) {
    return String(value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }
})();
