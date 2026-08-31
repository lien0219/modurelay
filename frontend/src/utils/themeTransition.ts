/** Apply the existing theme state without leaving a view-transition overlay behind. */
export function toggleThemeWithTransition(currentDark: boolean, _event?: MouseEvent): boolean {
  const nextDark = !currentDark
  const root = document.documentElement
  root.classList.toggle('dark', nextDark)
  localStorage.setItem('theme', nextDark ? 'dark' : 'light')
  return nextDark
}
