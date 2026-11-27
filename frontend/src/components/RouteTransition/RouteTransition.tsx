import { useEffect, type ReactNode } from 'react'
import { useLocation } from 'react-router-dom'

interface RouteTransitionProps {
  children: ReactNode
}

export function RouteTransition({ children }: RouteTransitionProps) {
  const location = useLocation()

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'instant' as ScrollBehavior })
  }, [location.pathname])

  return (
    <div key={location.pathname} className="bili-page-enter">
      {children}
    </div>
  )
}
