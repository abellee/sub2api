export const OPEN_USER_RESOURCE_EVENT = 'open-user-resource'

export function openUserResource(resource: 'points' | 'items') {
  window.dispatchEvent(new CustomEvent(OPEN_USER_RESOURCE_EVENT, { detail: resource }))
}
