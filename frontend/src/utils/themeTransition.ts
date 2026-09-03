/** Apply the existing theme state immediately so no full-page transition layer is created. */
export function toggleThemeWithTransition(currentDark: boolean, _event?: MouseEvent): boolean {
  const nextDark = !currentDark
  const root = document.documentElement
  root.classList.toggle('dark', nextDark)
  localStorage.setItem('theme', nextDark ? 'dark' : 'light')
  return nextDark
}
