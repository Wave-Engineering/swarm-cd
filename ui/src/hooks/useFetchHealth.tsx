import { useEffect, useState } from "react"

export interface HealthResponse {
  status: string
  booted_at: string
  version: string
  uptime_seconds: number
  update_interval_seconds: number
  stacks_managed: number
  mutation_api_enabled: boolean
  config_warnings?: string[]
}

const devHealthData: HealthResponse = {
  status: "healthy",
  booted_at: "2026-03-22T08:00:00Z",
  version: "0.1.0-dev",
  uptime_seconds: 7200,
  update_interval_seconds: 30,
  stacks_managed: 3,
  mutation_api_enabled: false,
  config_warnings: ["update_interval and compose override both set — compose value wins"]
}

async function fetchHealthFromServer(): Promise<HealthResponse> {
  const response = await fetch("/health")
  if (!response.ok) {
    throw new Error("Failed to fetch health status")
  }
  return (await response.json()) as HealthResponse
}

export default function useFetchHealth(intervalMs = 30000): {
  health: HealthResponse | null
  error: string | null
} {
  const [health, setHealth] = useState<HealthResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  const fetchHealth = async (): Promise<void> => {
    try {
      const data = import.meta.env.MODE === "development" ? devHealthData : await fetchHealthFromServer()
      setHealth(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : "An unknown error occurred")
    }
  }

  useEffect(() => {
    void fetchHealth()

    const intervalId = setInterval(fetchHealth, intervalMs)
    return () => clearInterval(intervalId)
  }, [intervalMs])

  return { health, error }
}
