export default {
  usageTiers: {
    title: 'Usage tiers', enabled: 'Enable usage tiers', dailyTokens: 'Daily token count', days: 'Consecutive qualifying days', missed: 'Allowed consecutive missed days', rate: 'Personal multiplier', add: 'Add tier', linked: 'Groups sharing qualification', excludedUsers: 'Excluded users', excludedHint: 'Search by username or email and select a user.',
    adminHint: 'Count input, output, cache creation and cache read tokens across this user’s keys in this group each Beijing calendar day, for all or selected models. Linked groups share the result; their tokens do not count toward this rule. Changing model scope, tier conditions or toggling the rule restarts progress. Updating links or exclusions preserves progress.',
    userHint: 'Input, output, cache creation and cache read tokens from selected models across all keys in the source group count each Beijing calendar day. Consecutive qualifying days unlock the multiplier immediately. The next tier starts the following day. A missed day breaks the upgrade streak.',
    priorityHint: 'The lowest multiplier applies when personal rates or other tier rules overlap. Independent image/video multipliers and peak rules follow the group settings.',
    rule: 'Tier {stage}: {tokens} tokens daily for {days} consecutive days → {rate}x; {missed} consecutive missed days cancel all tiers.',
    excluded: 'You do not participate in this rule.', active: 'Tier {stage} · {rate}x', today: 'Today {tokens} / {target} tokens', maintained: 'Qualified today; missed days reset', remaining: 'All tiers expire after {days} more missed days',
    next: 'Next tier {stage} · {rate}x: {days} / {required} consecutive days', nextProgress: 'Next tier progress', maintenance: 'Today’s maintenance progress', nextTomorrow: 'Upgraded; the next tier starts tomorrow', locked: 'Highest tier reached; upgrades locked. Daily qualification is still required.',
    modelScope: 'Models counted', allModels: 'All models in this group', selectedModels: 'Selected models', searchModels: 'Search models', unavailableUser: 'User unavailable', searchFailed: 'Could not search users. Try again.', noUsers: 'No matching users to select',
    saved: 'Usage tiers saved', loadFailed: 'Could not load usage tiers', saveFailed: 'Could not save usage tiers', invalid: 'Check token counts, models, days, multipliers and selected users. Token counts must be positive integers. Later tiers must have nondecreasing token counts and lower multipliers.',
  },
}
