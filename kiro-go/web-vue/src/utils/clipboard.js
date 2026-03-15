/**
 * 将文本复制到剪贴板，兼容 HTTP 和 HTTPS 环境
 * 优先使用 navigator.clipboard API，失败时回退到 execCommand
 */
export function copyToClipboard(text) {
  if (navigator.clipboard && window.isSecureContext) {
    return navigator.clipboard.writeText(text)
  }
  // fallback for HTTP
  const ta = document.createElement('textarea')
  ta.value = text
  ta.style.cssText = 'position:fixed;left:-9999px;top:-9999px'
  document.body.appendChild(ta)
  ta.select()
  document.execCommand('copy')
  document.body.removeChild(ta)
  return Promise.resolve()
}
