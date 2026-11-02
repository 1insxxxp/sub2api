export default {
  title: 'Model Probes',
  group: 'Group to probe', selectGroup: 'Select a group', searchGroups: 'Search group names',
  description: 'Probe public models through the group’s actual scheduling, using the same tests as account management. Probes call the upstream service without creating user bills. Results do not validate a specific user’s key permissions, balance or subscription.',
  probeAll: 'Probe all', stop: 'Stop probes', retryFailed: 'Retry unsuccessful', probe: 'Probe', retry: 'Retry', reload: 'Reload',
  checkingAccess: 'Checking access…', loading: 'Loading…',
  models: 'Models', completed: 'Completed', failures: 'Unsuccessful', progress: 'Probe progress', model: 'Model name', result: 'Probe result', duration: 'Duration',
  initial: 'Select a group to view and probe its public models', noModels: 'This group has no public models available to probe', noGroups: 'No groups are available to probe',
  status: { idle: 'Not probed', pending: 'Queued', testing: 'Probing', success: 'Available', failed: 'Unsuccessful', timeout: 'Timed out', cancelled: 'Stopped', unsupported: 'Unsupported' },
  reasons: { failed: 'This probe did not pass. Retry to check again.', timeout: 'This probe timed out. Try again later.', unsupported: 'Automatic probing is unavailable for this model, or additional media is required.' },
  errors: { forbidden: 'User Support access is not enabled. Contact the primary administrator.', groups: 'Could not load groups. Try again.', models: 'Could not load models for this group. Try again.' },
}
