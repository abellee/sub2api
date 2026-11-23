export default {
  usageTiers: {
    title: 'Usage tiers', enabled: 'Enable usage tiers', dailyCost: 'Daily amount charged ($)', days: 'Consecutive qualifying days', missed: 'Allowed consecutive missed days', rate: 'Personal multiplier', add: 'Add tier', linked: 'Groups sharing qualification', excludedUsers: 'Excluded user IDs', excludedHint: 'Separate IDs with commas, spaces or new lines.',
    adminHint: 'Count actual charges across this user’s keys in this group each Beijing calendar day. Linked groups share the result; their spend does not count toward this rule. Changing tier conditions or toggling the rule restarts progress. Updating links or exclusions preserves progress.',
    userHint: 'Actual charges across all keys in the source group count each Beijing calendar day. Consecutive qualifying days unlock the multiplier immediately. The next tier starts the following day. A missed day breaks the upgrade streak.',
    priorityHint: 'The lowest multiplier applies when personal rates or other tier rules overlap. Independent image/video multipliers and peak rules follow the group settings.',
    rule: 'Tier {stage}: ${cost} daily for {days} consecutive days → {rate}x; {missed} consecutive missed days cancel all tiers.',
    excluded: 'You do not participate in this rule.', active: 'Tier {stage} · {rate}x', today: 'Today ${cost} / ${target}', maintained: 'Qualified today; missed days reset', remaining: 'All tiers expire after {days} more missed days',
    next: 'Next tier {stage} · {rate}x: {days} / {required} consecutive days', nextProgress: 'Next tier progress', maintenance: 'Today’s maintenance progress', nextTomorrow: 'Upgraded; the next tier starts tomorrow', locked: 'Highest tier reached; upgrades locked. Daily qualification is still required.',
    saved: 'Usage tiers saved', loadFailed: 'Could not load usage tiers', saveFailed: 'Could not save usage tiers', invalid: 'Check amounts, days, multipliers and user IDs. Later tiers must have nondecreasing amounts and lower multipliers.',
  },
}
