import DOMPurify from 'dompurify'

// SMIL 动画标签（animate/set）和 use 在 DOMPurify 的 svg profile 里会被丢掉。
// 进度条动画靠 <use href="#id"> 切片加上 CSS，所以放行这些标签。
// use 只允许页内片段引用，避免拉外部 SVG。animate 的 attributeName 不能指向事件或链接。
const EXTRA_SVG_TAGS = ['animate', 'set', 'use']
const SMIL_TAGS = ['animate', 'set']
const UNSAFE_ANIMATION_TARGET = /^(on|href$|xlink:href$)/i
const FRAGMENT_REF = /^#[A-Za-z_][\w:.-]*$/

DOMPurify.addHook('uponSanitizeAttribute', (node, data) => {
  const tag = node.nodeName.toLowerCase()
  if (tag === 'use' && (data.attrName === 'href' || data.attrName === 'xlink:href')) {
    if (!FRAGMENT_REF.test(data.attrValue.trim())) data.keepAttr = false
    return
  }
  if (!SMIL_TAGS.includes(tag)) return
  if (data.attrName === 'attributename' && UNSAFE_ANIMATION_TARGET.test(data.attrValue.trim())) {
    data.keepAttr = false
  }
})

export function sanitizeSvg(svg: string): string {
  if (!svg) return ''
  return DOMPurify.sanitize(svg, {
    USE_PROFILES: { svg: true, svgFilters: true },
    ADD_TAGS: EXTRA_SVG_TAGS
  })
}
