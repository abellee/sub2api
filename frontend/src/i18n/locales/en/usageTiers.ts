export default {
  usageTiers: {
    title: 'Usage tiers', enabled: 'Enable usage tiers', mode: 'Qualification mode', tokenMode: 'Daily token count', costMode: 'Daily actual spend', dailyCost: 'Daily actual spend ($)', dailyTokens: 'Daily token count', days: 'Consecutive qualifying days', missed: 'Allowed consecutive missed days', rate: 'Personal multiplier', add: 'Add tier', linked: 'Groups sharing qualification', excludedUsers: 'Excluded users', excludedHint: 'Search by username or email and select a user.',
    adminHint: 'Choose daily actual spend or daily tokens (input, output, cache creation and cache read). Both modes count all or selected models across this user’s keys in this group each Beijing calendar day. Linked groups share the result; their usage does not count toward this rule. Changing mode, model scope, tier conditions or toggling the rule restarts progress. Updating links or exclusions preserves progress.',
    userHint: 'Input, output, cache creation and cache read tokens from selected models across all keys in the source group count each Beijing calendar day. Consecutive qualifying days unlock the multiplier immediately. The next tier starts the following day. A missed day breaks the upgrade streak.',
    costUserHint: 'Actual spend on selected models across all keys in the source group counts each Beijing calendar day. Consecutive qualifying days unlock the multiplier immediately. The next tier starts the following day. A missed day breaks the upgrade streak.',
    costRule: 'Tier {stage}: ${cost} actual spend daily for {days} consecutive days → {rate}x; {missed} consecutive missed days cancel all tiers.',
    costToday: 'Today ${cost} / ${target}',
    priorityHint: 'The lowest multiplier applies when personal rates or other tier rules overlap. Independent image/video multipliers and peak rules follow the group settings.',
    rule: 'Tier {stage}: {tokens} tokens daily for {days} consecutive days → {rate}x; {missed} consecutive missed days cancel all tiers.',
    excluded: 'You do not participate in this rule.', active: 'Tier {stage} · {rate}x', today: 'Today {tokens} / {target} tokens', maintained: 'Qualified today; missed days reset', remaining: 'All tiers expire after {days} more missed days',
    next: 'Next tier {stage} · {rate}x: {days} / {required} consecutive days', nextProgress: 'Next tier progress', maintenance: 'Today’s maintenance progress', nextTomorrow: 'Upgraded; the next tier starts tomorrow', locked: 'Highest tier reached; upgrades locked. Daily qualification is still required.',
    modelScope: 'Models counted', allModels: 'All models in this group', selectedModels: 'Selected models', searchModels: 'Search models', unavailableUser: 'User unavailable', searchFailed: 'Could not search users. Try again.', noUsers: 'No matching users to select',
    saved: 'Usage tiers saved', loadFailed: 'Could not load usage tiers', saveFailed: 'Could not save usage tiers', invalid: 'Check amounts or token counts, models, days, multipliers and selected users. Amounts must be positive; token counts must be positive integers. Later tiers must have nondecreasing targets and lower multipliers.',
  },
}
