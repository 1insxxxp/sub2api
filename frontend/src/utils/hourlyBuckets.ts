export function formatLocalHourlyBucket(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:00`
}

export function getLatestHourlyBuckets(count: number, now: Date = new Date()): string[] {
  const currentHour = new Date(now)
  currentHour.setMinutes(0, 0, 0)
  return Array.from({ length: count }, (_, index) => {
    const bucket = new Date(currentHour)
    bucket.setHours(currentHour.getHours() - (count - 1 - index))
    return formatLocalHourlyBucket(bucket)
  })
}
