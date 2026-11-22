import { referencePages } from './reference-content.js';
import { documentCatalog } from './catalog.js';
import { documentMarkdown } from './documentation.js';

const $ = (selector) => document.querySelector(selector);
const escape = (value) => String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
const inline = (value) => escape(value).replace(/`([^`]+)`/g, '<code>$1</code>');
let current;

function highlight(value) {
  const pattern = /"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|#[^\n]*|\/\/[^\n]*|\b\d+\b/g;
  let result = '', cursor = 0;
  for (const match of value.matchAll(pattern)) {
    result += escape(value.slice(cursor, match.index));
    const token = match[0];
    const type = /^["']/.test(token) ? (/^\s*:/.test(value.slice(match.index + token.length)) ? 'key' : 'string') : /^\d/.test(token) ? 'number' : 'comment';
    result += `<span class="syntax-${type}">${escape(token)}</span>`;
    cursor = match.index + token.length;
  }
  return result + escape(value.slice(cursor));
}

function renderBlock(block) {
  if (block.type === 'paragraph') return `<p>${inline(block.value)}</p>`;
  if (block.type === 'note') return `<div class="doc-note">${inline(block.value)}</div>`;
  if (block.type === 'table') return `<div class="table-scroll"><table><thead><tr>${block.headers.map(value => `<th scope="col">${inline(value)}</th>`).join('')}</tr></thead><tbody>${block.rows.map(row => `<tr>${row.map(value => `<td>${inline(value)}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`;
  if (block.type === 'list') return `<ul>${block.items.map(item => `<li>${inline(item)}</li>`).join('')}</ul>`;
  if (block.type === 'code') return `<div class="doc-code"><header><span>${escape(block.lang || 'text')}</span><button type="button" data-copy-block>复制代码</button></header><pre tabindex="0"><code>${highlight(block.value)}</code></pre></div>`;
  if (block.type === 'links') return `<p class="related-documents">相关文档：${block.items.map(item => escape(item.label)).join(' · ')}。请从左侧选择已关联的分组。</p>`;
  return '';
}

function selectDocument(selection) {
  const page = referencePages.find(page => page.id === selection?.documentId);
  if (!page) return;
  current = page;
  const metadata = documentCatalog.find(doc => doc.id === page.id);
  const groupName = typeof selection.groupName === 'string' ? selection.groupName.trim().slice(0, 200) : '';
  const groups = [];
  for (const block of page.blocks) {
    if (block.type === 'heading') groups.push({ title: block.value.replace(/^\d+\.\s*/, ''), blocks: [] });
    else if (groups.length) groups.at(-1).blocks.push(block);
  }
  $('#reference-title').textContent = groupName || page.title;
  $('#reference-description').textContent = page.lead;
  $('#document-kind').textContent = metadata?.kind === 'video' ? 'VIDEO / 视频生成' : 'IMAGE / 图片生成';
  $('#full-document').innerHTML = groups.map((group, index) => `<section class="doc-section" id="doc-${page.id}-${index + 1}" data-doc-index="${escape(group.title)}" aria-labelledby="heading-${index + 1}"><h3 id="heading-${index + 1}"><span>${String(index + 1).padStart(2, '0')}</span>${escape(group.title)}</h3><div class="doc-body">${group.blocks.map(renderBlock).join('')}</div></section>`).join('');
  $('#copy-status').textContent = '';
  document.dispatchEvent(new CustomEvent('creative-docs:rendered'));
}

document.addEventListener('creative-docs:select', event => selectDocument(event.detail));
document.addEventListener('click', async event => {
  const button = event.target.closest('button');
  if (!button) return;
  const code = button.hasAttribute('data-copy-block') ? button.closest('.doc-code').querySelector('code') : null;
  const text = code?.textContent;
  if (!text) return;
  try {
    await navigator.clipboard.writeText(text);
    $('#copy-status').textContent = '代码已复制。';
  } catch {
    const range = document.createRange();
    range.selectNodeContents(code);
    const selection = window.getSelection();
    selection.removeAllRanges();
    selection.addRange(range);
    $('#copy-status').textContent = '浏览器未允许自动复制，已选中代码，请手动复制。';
  }
});

$('#export-doc').addEventListener('click', () => {
  const url = URL.createObjectURL(new Blob([documentMarkdown(current)], { type: 'text/markdown;charset=utf-8' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = `TokenFree-${current.id}.md`;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
});

selectDocument({ documentId: new URL(location.href).searchParams.get('document') || documentCatalog[0].id });
