(function () {
  const PANEL_OPEN_KEY = 'itbsAiPanelOpen';
  const PANEL_MAXIMIZED_KEY = 'itbsAiPanelMaximized';
  const WRITING_ACTIONS = ['compose', 'reply'];
  let lastAssistantText = '';

  try {
    if (window.top !== window.self) return;
  } catch (err) {
    return;
  }

  if (window.__itbsAiAssistantLoaded) return;
  window.__itbsAiAssistantLoaded = true;

  function ready(fn) {
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', fn);
    else fn();
  }

  ready(function () {
    if (!window.rcmail || !rcmail.env || !rcmail.env.itbs_ai_enabled) return;

    document.querySelectorAll('#itbs-ai-fab, #itbs-ai-panel, #itbs-ai-compose-shortcut').forEach(function (node) {
      node.remove();
    });

    const fab = document.createElement('button');
    fab.id = 'itbs-ai-fab';
    fab.type = 'button';
    fab.setAttribute('aria-label', 'Open PNP Mail AI');
    fab.innerHTML = '<span>AI</span>';

    const panel = document.createElement('section');
    panel.id = 'itbs-ai-panel';
    panel.setAttribute('aria-label', 'PNP Mail AI chat');
    panel.innerHTML = `
      <div class="itbs-ai-header">
        <strong>${escapeHtml(rcmail.env.itbs_ai_title || 'PNP Mail AI')}</strong>
        <div class="itbs-ai-window-actions">
          <button type="button" id="itbs-ai-maximize" title="Maximize AI chat" aria-label="Maximize AI chat">□</button>
          <button type="button" id="itbs-ai-close" title="Close AI chat" aria-label="Close AI chat">&times;</button>
        </div>
      </div>
      <div class="itbs-ai-body">
        <div id="itbs-ai-messages" aria-live="polite">
          <div class="itbs-ai-message assistant">Ask about the open email, recent inbox, or what you want to write. I will not send or change mail without your confirmation.</div>
        </div>
        <div class="itbs-ai-suggestions" aria-label="Suggested AI actions">
          <button type="button" data-ai="summarize">Summarize email</button>
          <button type="button" data-ai="reply">Draft reply</button>
          <button type="button" data-ai="recent">Recent emails</button>
        </div>
        <div class="itbs-ai-options">
          <select id="itbs-ai-language" title="Response language">
            <option value="English">English</option>
            <option value="Tagalog">Tagalog</option>
            <option value="Filipino">Filipino</option>
          </select>
          <select id="itbs-ai-tone" title="Writing tone">
            <option value="professional">Professional</option>
            <option value="friendly">Friendly</option>
            <option value="formal">Formal</option>
            <option value="short and direct">Short and direct</option>
          </select>
        </div>
        <div class="itbs-ai-composer">
          <textarea id="itbs-ai-prompt" rows="2" placeholder="Message PNP Mail AI..."></textarea>
          <button type="button" id="itbs-ai-send" title="Send">Send</button>
        </div>
        <div class="itbs-ai-footer">
          <button type="button" id="itbs-ai-insert">Use in composer</button>
          <span id="itbs-ai-status"></span>
        </div>
      </div>`;

    document.body.appendChild(fab);
    document.body.appendChild(panel);

    setPanelOpen(panel, localStorage.getItem(PANEL_OPEN_KEY) === '1');
    setPanelMaximized(panel, localStorage.getItem(PANEL_MAXIMIZED_KEY) === '1');
    installComposeShortcut(panel);

    fab.addEventListener('click', function () {
      setPanelOpen(panel, true);
      panel.querySelector('#itbs-ai-prompt').focus();
    });

    panel.querySelector('#itbs-ai-close').addEventListener('click', function () {
      setPanelOpen(panel, false);
    });

    panel.querySelector('#itbs-ai-maximize').addEventListener('click', function () {
      setPanelMaximized(panel, !panel.classList.contains('is-maximized'));
    });

    panel.querySelectorAll('[data-ai]').forEach(function (btn) {
      btn.addEventListener('click', function () {
        const action = this.getAttribute('data-ai');
        if (action === 'recent') {
          const prompt = panel.querySelector('#itbs-ai-prompt');
          prompt.value = 'What are my recent emails?';
          callAI('chat');
          return;
        }
        callAI(action);
      });
    });

    panel.querySelector('#itbs-ai-send').addEventListener('click', function () {
      callAI(detectActionFromPrompt(panel.querySelector('#itbs-ai-prompt').value));
    });

    panel.querySelector('#itbs-ai-prompt').addEventListener('keydown', function (event) {
      if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        callAI(detectActionFromPrompt(this.value));
      }
    });

    panel.querySelector('#itbs-ai-insert').addEventListener('click', function () {
      if (!lastAssistantText.trim()) {
        setStatus('Nothing to insert yet.');
        return;
      }

      if (insertIntoCompose(lastAssistantText)) {
        setStatus('Inserted into composer.');
      } else {
        setStatus('Open the compose editor first.');
      }
    });
  });

  function setPanelOpen(panel, open) {
    panel.classList.toggle('is-open', open);
    localStorage.setItem(PANEL_OPEN_KEY, open ? '1' : '0');
  }

  function setPanelMaximized(panel, maximized) {
    panel.classList.toggle('is-maximized', maximized);
    localStorage.setItem(PANEL_MAXIMIZED_KEY, maximized ? '1' : '0');
    const button = panel.querySelector('#itbs-ai-maximize');
    if (button) {
      button.textContent = maximized ? '▣' : '□';
      button.setAttribute('title', maximized ? 'Restore AI chat' : 'Maximize AI chat');
      button.setAttribute('aria-label', maximized ? 'Restore AI chat' : 'Maximize AI chat');
    }
  }

  function installComposeShortcut(panel) {
    if (!hasComposeEditor() || document.getElementById('itbs-ai-compose-shortcut')) return;

    const target = document.querySelector('.tox-toolbar, .editor-toolbar, #composebody') || document.querySelector('form');
    if (!target || !target.parentNode) return;

    const button = document.createElement('button');
    button.id = 'itbs-ai-compose-shortcut';
    button.type = 'button';
    button.textContent = 'AI';
    button.title = 'Open PNP Mail AI';
    button.addEventListener('click', function () {
      setPanelOpen(panel, true);
      panel.querySelector('#itbs-ai-prompt').focus();
    });

    target.parentNode.insertBefore(button, target);
  }

  async function callAI(action) {
    const promptInput = document.getElementById('itbs-ai-prompt');
    const prompt = (promptInput.value || '').trim();
    const language = document.getElementById('itbs-ai-language').value || 'English';
    const tone = document.getElementById('itbs-ai-tone').value || 'professional';
    const clientContext = getClientContext();
    const serverContext = await fetchMailboxContext();
    const context = {
      page: clientContext,
      mailbox: serverContext
    };
    const emailText = buildTextContext(clientContext, serverContext);

    if (requiresEmailContext(action) && !emailText.trim()) {
      setStatus('Open an email or select text first.');
      return;
    }

    appendMessage('user', labelForAction(action, prompt));
    const assistantMessage = appendMessage('assistant', '');
    lastAssistantText = '';
    setStatus('Streaming...');

    try {
      const form = new FormData();
      form.append('_token', rcmail.env.request_token || '');
      form.append('_ai_action', action);
      form.append('_stream', '1');
      form.append('_text', emailText);
      form.append('_prompt', prompt);
      form.append('_language', language);
      form.append('_tone', tone);
      form.append('_subject', clientContext.subject || '');
      form.append('_context', JSON.stringify(context));

      const response = await fetch(rcmail.url('plugin.itbs_ai'), {
        method: 'POST',
        body: form,
        credentials: 'same-origin'
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || 'AI request failed.');
      }

      if (!response.body || !window.TextDecoder) {
        updateAssistantMessage(assistantMessage, await response.text());
      } else {
        await readStream(response, assistantMessage);
      }

      const shouldInsert = WRITING_ACTIONS.includes(action);
      if (shouldInsert && lastAssistantText.trim()) {
        const draftText = cleanDraftForCompose(lastAssistantText);
        setStatus(insertIntoCompose(draftText) ? 'Inserted into composer.' : 'Draft ready. Open compose to insert.');
      } else {
        setStatus(response.headers.get('X-ITBS-AI-Mock') === 'true' ? 'Mock result.' : 'Done.');
      }

      promptInput.value = '';
    } catch (err) {
      updateAssistantMessage(assistantMessage, 'Error: ' + err.message);
      setStatus('Error');
    }
  }

  async function fetchMailboxContext() {
    try {
      const form = new FormData();
      form.append('_token', rcmail.env.request_token || '');
      form.append('_folder', getCurrentFolder());

      const response = await fetch(rcmail.url('plugin.itbs_ai_context'), {
        method: 'POST',
        body: form,
        credentials: 'same-origin'
      });

      if (!response.ok) return {};
      return await response.json();
    } catch (err) {
      return {};
    }
  }

  async function readStream(response, assistantMessage) {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();

    while (true) {
      const read = await reader.read();
      if (read.done) break;
      updateAssistantMessage(assistantMessage, lastAssistantText + decoder.decode(read.value, { stream: true }));
    }

    const tail = decoder.decode();
    if (tail) updateAssistantMessage(assistantMessage, lastAssistantText + tail);
  }

  function appendMessage(role, text) {
    const messages = document.getElementById('itbs-ai-messages');
    const message = document.createElement('div');
    message.className = 'itbs-ai-message ' + role;
    renderMessage(message, role, text);
    messages.appendChild(message);
    messages.scrollTop = messages.scrollHeight;
    return message;
  }

  function updateAssistantMessage(message, text) {
    lastAssistantText = text;
    renderMessage(message, 'assistant', text);
    const messages = document.getElementById('itbs-ai-messages');
    messages.scrollTop = messages.scrollHeight;
  }

  function renderMessage(message, role, text) {
    if (role === 'assistant') {
      message.innerHTML = renderMarkdown(text || '');
    } else {
      message.textContent = text || '';
    }
  }

  function renderMarkdown(value) {
    const lines = String(value || '').replace(/\r\n/g, '\n').split('\n');
    const blocks = [];
    let paragraph = [];
    let list = null;
    let code = [];
    let inCode = false;

    function flushParagraph() {
      if (!paragraph.length) return;
      blocks.push('<p>' + paragraph.map(formatInline).join('<br>') + '</p>');
      paragraph = [];
    }

    function flushList() {
      if (!list) return;
      blocks.push('<' + list.type + '>' + list.items.map(function (item) {
        return '<li>' + formatInline(item) + '</li>';
      }).join('') + '</' + list.type + '>');
      list = null;
    }

    function flushCode() {
      if (!code.length) return;
      blocks.push('<pre><code>' + escapeHtml(code.join('\n')) + '</code></pre>');
      code = [];
    }

    lines.forEach(function (line) {
      if (/^```/.test(line.trim())) {
        if (inCode) {
          flushCode();
          inCode = false;
        } else {
          flushParagraph();
          flushList();
          inCode = true;
        }
        return;
      }

      if (inCode) {
        code.push(line);
        return;
      }

      const ordered = line.match(/^\s*\d+\.\s+(.*)$/);
      const unordered = line.match(/^\s*[-*]\s+(.*)$/);
      if (ordered || unordered) {
        flushParagraph();
        const type = ordered ? 'ol' : 'ul';
        if (!list || list.type !== type) {
          flushList();
          list = { type: type, items: [] };
        }
        list.items.push((ordered || unordered)[1]);
        return;
      }

      if (!line.trim()) {
        flushParagraph();
        flushList();
        return;
      }

      flushList();
      paragraph.push(line);
    });

    flushParagraph();
    flushList();
    flushCode();
    return blocks.join('') || '<p></p>';
  }

  function formatInline(value) {
    return escapeHtml(value)
      .replace(/`([^`]+)`/g, '<code>$1</code>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(/\*([^*]+)\*/g, '<em>$1</em>');
  }

  function setStatus(text) {
    const status = document.getElementById('itbs-ai-status');
    if (status) status.textContent = text;
  }

  function detectActionFromPrompt(prompt) {
    const value = String(prompt || '').toLowerCase();
    if (/\b(draft|reply|respond)\b/.test(value) && hasComposeEditor()) return 'reply';
    if (/\b(compose|write an email|new email)\b/.test(value)) return 'compose';
    return 'chat';
  }

  function labelForAction(action, prompt) {
    const labels = {
      summarize: 'Summarize this email.',
      reply: prompt || 'Draft a reply.',
      compose: prompt || 'Compose an email.',
      translate: prompt || 'Translate this email.',
      phishing: 'Check this email for risk.',
      chat: prompt || 'Help me with my mailbox.'
    };
    return labels[action] || prompt || action;
  }

  function requiresEmailContext(action) {
    return ['reply', 'summarize', 'translate', 'phishing'].includes(action);
  }

  function getClientContext() {
    return {
      folder: getCurrentFolder(),
      subject: getSubject(),
      selectedText: String(window.getSelection ? window.getSelection() : '').trim(),
      composeDraft: getComposeText(),
      openEmail: getMessageText(),
      selectedMessage: getSelectedMessageRowText(),
      visibleMessages: getVisibleMessageRows()
    };
  }

  function buildTextContext(clientContext, serverContext) {
    const parts = [];
    if (clientContext.selectedText) parts.push('Selected text:\n' + clientContext.selectedText);
    if (clientContext.subject) parts.push('Subject: ' + clientContext.subject);
    if (clientContext.openEmail) parts.push('Open email:\n' + clientContext.openEmail);
    if (clientContext.composeDraft) parts.push('Compose draft:\n' + clientContext.composeDraft);
    if (clientContext.selectedMessage) parts.push('Selected mailbox row:\n' + clientContext.selectedMessage);
    if (clientContext.visibleMessages.length) parts.push('Visible message list:\n' + clientContext.visibleMessages.join('\n'));
    if (serverContext && Array.isArray(serverContext.recent_messages) && serverContext.recent_messages.length) {
      parts.push('Recent mailbox messages:\n' + serverContext.recent_messages.map(formatMessageSummary).join('\n'));
    }
    return uniqueText(parts).join('\n\n').trim();
  }

  function formatMessageSummary(message, index) {
    return [
      String(index + 1) + '.',
      message.subject ? 'Subject: ' + message.subject : '',
      message.from ? 'From: ' + message.from : '',
      message.date ? 'Date: ' + message.date : ''
    ].filter(Boolean).join(' ');
  }

  function hasComposeEditor() {
    return Boolean(document.querySelector('#composebody, iframe#composebody_ifr, iframe.tox-edit-area__iframe, iframe#mceu_0_ifr'));
  }

  function getComposeText() {
    const editor = getTinyMceEditor();
    if (editor && typeof editor.getContent === 'function') {
      return cleanText(editor.getContent({ format: 'text' }) || '');
    }

    const compose = document.querySelector('#composebody');
    if (compose && compose.value) return cleanText(compose.value);
    return getIframeText('iframe#composebody_ifr, iframe.tox-edit-area__iframe, iframe#mceu_0_ifr');
  }

  function getMessageText() {
    const selectors = ['#message-header', '#messagebody', '#message-content', '.message-htmlpart', '.message-text', '.message-part', '.message-partheaders'];
    const parts = [];

    selectors.forEach(function (selector) {
      document.querySelectorAll(selector).forEach(function (node) {
        const text = cleanText(node.innerText || node.textContent || '');
        if (text) parts.push(text);
      });
    });

    document.querySelectorAll('iframe').forEach(function (iframe) {
      if (iframe.id === 'composebody_ifr' || iframe.classList.contains('tox-edit-area__iframe')) return;
      const text = getIframeText(iframe);
      if (text) parts.push(text);
    });

    return uniqueText(parts).join('\n\n');
  }

  function getIframeText(target) {
    let frames = [];
    if (typeof target === 'string') frames = Array.from(document.querySelectorAll(target));
    else if (target && target.contentDocument) frames = [target];

    const parts = [];
    frames.forEach(function (iframe) {
      try {
        if (iframe.contentDocument && iframe.contentDocument.body) {
          const text = cleanText(iframe.contentDocument.body.innerText || iframe.contentDocument.body.textContent || '');
          if (text) parts.push(text);
        }
      } catch (err) {
        // Cross-origin frames are ignored; Roundcube message frames are same-origin.
      }
    });

    return uniqueText(parts).join('\n\n');
  }

  function getSelectedMessageRowText() {
    const row = document.querySelector('.messagelist tr.selected, .messagelist li.selected, #messagelist tr.selected, #messagelist li.selected');
    return row ? cleanText(row.innerText || row.textContent || '') : '';
  }

  function getVisibleMessageRows() {
    return Array.from(document.querySelectorAll('.messagelist tr, .messagelist li, #messagelist tr, #messagelist li'))
      .map(function (row) {
        return cleanText(row.innerText || row.textContent || '');
      })
      .filter(Boolean)
      .slice(0, 12);
  }

  function getSubject() {
    const subjectInput = document.querySelector('input[name="_subject"], #compose-subject');
    if (subjectInput && subjectInput.value) return cleanText(subjectInput.value);
    const subjectView = document.querySelector('#message-header .subject, .subject, h2.subject, .messagelist tr.selected .subject a, .messagelist li.selected .subject');
    return subjectView ? cleanText(subjectView.innerText || subjectView.textContent || '') : '';
  }

  function getCurrentFolder() {
    if (rcmail.env && rcmail.env.mailbox) return rcmail.env.mailbox;
    const selected = document.querySelector('#mailboxlist li.selected a, #mailboxlist a.selected, .folderlist li.selected a');
    return selected ? cleanText(selected.getAttribute('rel') || selected.getAttribute('data-id') || selected.textContent || 'INBOX') : 'INBOX';
  }

  function getTinyMceEditor() {
    if (!window.tinymce) return null;

    const candidates = ['composebody', 'composebody_ifr'];
    for (let i = 0; i < candidates.length; i += 1) {
      const editor = window.tinymce.get(candidates[i]);
      if (editor && !editor.removed) return editor;
    }

    if (window.tinymce.activeEditor && !window.tinymce.activeEditor.removed) {
      const active = window.tinymce.activeEditor;
      const target = active.getElement ? active.getElement() : null;
      if (!target || target.id === 'composebody') return active;
    }

    const editors = window.tinymce.editors || [];
    for (let i = 0; i < editors.length; i += 1) {
      const editor = editors[i];
      const target = editor && editor.getElement ? editor.getElement() : null;
      if (editor && !editor.removed && target && target.id === 'composebody') return editor;
    }

    return null;
  }

  function insertIntoCompose(value) {
    if (!value) return false;

    const editor = getTinyMceEditor();
    if (editor && typeof editor.insertContent === 'function') {
      editor.focus();
      editor.insertContent(textToHtml(value));
      if (typeof editor.save === 'function') editor.save();
      syncComposeTextarea();
      return true;
    }

    const compose = document.querySelector('#composebody');
    if (compose) {
      compose.value = compose.value ? compose.value + '\n\n' + value : value;
      syncComposeTextarea();
      return true;
    }

    return false;
  }

  function cleanDraftForCompose(value) {
    return String(value || '')
      .replace(/^```[a-z]*\s*/i, '')
      .replace(/```$/i, '')
      .replace(/^\s*(draft reply|reply|email body|subject)\s*:\s*/gim, '')
      .replace(/^\s*to reply[^:\n]*:\s*/gim, '')
      .replace(/^\s*the draft[^:\n]*:\s*/gim, '')
      .replace(/^\s*please confirm.*roundcube.*$/gim, '')
      .replace(/^\s*if you need.*$/gim, '')
      .trim();
  }

  function syncComposeTextarea() {
    const compose = document.querySelector('#composebody');
    if (!compose) return;
    compose.dispatchEvent(new Event('input', { bubbles: true }));
    compose.dispatchEvent(new Event('change', { bubbles: true }));
  }

  function textToHtml(value) {
    return '<p>' + escapeHtml(value).replace(/\n{2,}/g, '</p><p>').replace(/\n/g, '<br>') + '</p>';
  }

  function uniqueText(values) {
    const seen = new Set();
    return values.map(cleanText).filter(function (value) {
      if (!value || seen.has(value)) return false;
      seen.add(value);
      return true;
    });
  }

  function cleanText(value) {
    return String(value || '').replace(/\s+\n/g, '\n').replace(/\n{3,}/g, '\n\n').replace(/[ \t]{2,}/g, ' ').trim();
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
