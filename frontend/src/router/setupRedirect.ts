export function resolveCompletedSetupRedirectPath(
  isAuthenticated: boolean,
  isAdmin: boolean,
  canAccessAdminWorkbench = false
): string {
  if (!isAuthenticated) {
    return '/login'
  }

  if (isAdmin) {
    return '/admin/dashboard'
  }
  if (canAccessAdminWorkbench) {
    return '/admin/workbench'
  }
  return '/dashboard'
}
