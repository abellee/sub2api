import { expect, it } from 'vitest'
import { resolveDocumentationGroups } from '../creativeDocumentation'

it('resolves documents by saved ID, keeps duplicate bindings and hides unbound or unknown documents', () => {
  const groups = [
    { id: 1, name: 'Renamed image group' },
    { id: 2, name: 'Another image route' },
    { id: 3, name: 'Video group' },
    { id: 4, name: 'Unbound' },
    { id: 5, name: 'Unknown document' },
  ]
  expect(resolveDocumentationGroups(groups, { 1: 'banana', 2: 'banana', 3: 'sd-video', 5: 'bad', 999: 'banana' }))
    .toMatchObject([
      { id: '1', name: 'Renamed image group', documentId: 'banana', kind: 'image' },
      { id: '2', name: 'Another image route', documentId: 'banana', kind: 'image' },
      { id: '3', documentId: 'sd-video', kind: 'video' },
    ])
})
