import { test } from 'node:test'
import assert from 'node:assert/strict'
import { referencePages, navigation } from '../public/creative-docs/reference-content.js'
import { pages } from '../public/creative-docs/content.js'
import { agentPrompt } from '../public/creative-docs/documentation.js'

test('all six groups and four standalone documents have reachable, complete content', () => {
  const ids = navigation.flatMap(item => item.pages ?? [item.page])
  assert.deepEqual(ids, ['image-1k', 'banana', 'image-az', 'image-adobe', 'image-official', 'image2', 'seedream', 'mj-v8.2', 'video', 'sd-video'])
  assert.equal(new Set(ids).size, referencePages.length)
  const supported = new Set(['heading', 'paragraph', 'table', 'note', 'code', 'list', 'links'])
  for (const id of ids) {
    const page = referencePages.find(item => item.id === id)
    assert.ok(page?.blocks.some(block => block.type === 'code'), `${id} needs request examples`)
    assert.ok(page.blocks.some(block => block.type === 'heading'), `${id} needs sections`)
    assert.ok(page.blocks.every(block => supported.has(block.type)), `${id} contains unsupported content`)
    for (const block of page.blocks.filter(block => block.type === 'links')) {
      assert.ok(block.items.every(link => ids.includes(link.id)), `${id} contains an unreachable link`)
    }
  }
})

test('imported API examples consistently use the configured origin and local token variable', () => {
  const content = JSON.stringify(referencePages)
  assert.doesNotMatch(content, /api\.zzlye\.xyz|WENYUN_/)
  for (const page of referencePages) {
    assert.match(JSON.stringify(page.blocks), /https:\/\/please\.dontblockme\.men/)
  }
})

test('Base URL uses the origin while endpoint paths include the version once', () => {
  let bases = 0
  for (const page of [...referencePages, ...pages]) {
    for (const block of page.blocks.filter(block => block.type === 'table')) {
      for (const [label, value] of block.rows) {
        if (!/base\s*url/i.test(label)) continue
        bases++
        assert.equal(value.replaceAll('`', ''), 'https://please.dontblockme.men', page.id)
        assert.ok(agentPrompt(page, '示例分组').includes(value), page.id)
      }
    }
    assert.doesNotMatch(JSON.stringify(page), /\/v1\/v1\//)
  }
  assert.ok(bases > 0)
})
