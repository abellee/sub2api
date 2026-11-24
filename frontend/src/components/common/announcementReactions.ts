// Stable IDs: keep existing mappings when adding or reordering emoji.
export const announcementEmojis = [
  { id: 1, name: 'thumbs_up', emoji: '👍' },
  { id: 2, name: 'heart', emoji: '❤️' },
  { id: 3, name: 'laugh', emoji: '😂' },
  { id: 4, name: 'party', emoji: '🎉' },
  { id: 5, name: 'fire', emoji: '🔥' },
  { id: 6, name: 'wow', emoji: '😮' },
  { id: 7, name: 'sad', emoji: '😢' },
  { id: 8, name: 'thumbs_down', emoji: '👎' },
  { id: 9, name: 'swear', emoji: '🤬' },
  { id: 10, name: 'clap', emoji: '👏' },
  { id: 11, name: 'angry', emoji: '😠' },
  { id: 12, name: 'think', emoji: '🤔' },
  { id: 13, name: 'pray', emoji: '🙏' },
  { id: 14, name: 'rocket', emoji: '🚀' },
  { id: 15, name: 'eyes', emoji: '👀' },
  { id: 16, name: 'cool', emoji: '😎' },
  { id: 17, name: 'cry', emoji: '😭' },
  { id: 18, name: 'hundred', emoji: '💯' },
  { id: 19, name: 'sleep', emoji: '😴' },
  { id: 20, name: 'handshake', emoji: '🤝' },
  { id: 21, name: 'rage', emoji: '😡' },
  { id: 22, name: 'huff', emoji: '😤' },
  { id: 23, name: 'eye_roll', emoji: '🙄' },
  { id: 24, name: 'exploding_head', emoji: '🤯' },
  { id: 25, name: 'poop', emoji: '💩' },
  { id: 26, name: 'clown', emoji: '🤡' },
  { id: 27, name: 'unamused', emoji: '😒' },
  { id: 28, name: 'nauseated', emoji: '🤢' },
  { id: 29, name: 'vomit', emoji: '🤮' },
  { id: 30, name: 'fist', emoji: '👊' },
] as const

export type AnnouncementEmoji = typeof announcementEmojis[number]['id']
export type AnnouncementEmojiName = typeof announcementEmojis[number]['name']
export interface AnnouncementReaction {
  emoji_id: AnnouncementEmoji
  count: number
  reacted: boolean
}
