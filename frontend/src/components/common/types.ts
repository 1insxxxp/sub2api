/**
 * Common component types
 */

export interface Column {
  key: string
  label: string
  sortable?: boolean
  class?: string
  /** Exclude this column from mobile cards while preserving it in the desktop table. */
  mobileHidden?: boolean
  formatter?: (value: any, row: any) => string
}
