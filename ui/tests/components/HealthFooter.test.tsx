import { render, screen } from "@testing-library/react"
import HealthFooter from "../../src/components/HealthFooter"
import { HealthResponse } from "../../src/hooks/useFetchHealth"
import { formatUptime } from "../../src/utils/formatUptime"

describe("HealthFooter", () => {
  const health: HealthResponse = {
    status: "healthy",
    booted_at: "2026-03-22T08:00:00Z",
    version: "1.2.3",
    uptime_seconds: 7200,
    update_interval_seconds: 30,
    stacks_managed: 5,
    mutation_api_enabled: false
  }

  it("should render version", () => {
    render(<HealthFooter health={health} />)
    expect(screen.getByText("1.2.3")).toBeInTheDocument()
  })

  it("should render stacks managed count", () => {
    render(<HealthFooter health={health} />)
    expect(screen.getByText("5")).toBeInTheDocument()
  })

  it("should render uptime", () => {
    render(<HealthFooter health={health} />)
    expect(screen.getByText("2h")).toBeInTheDocument()
  })

  it("should render booted at", () => {
    render(<HealthFooter health={health} />)
    expect(screen.getByText(/Booted:/)).toBeInTheDocument()
  })
})

describe("formatUptime", () => {
  it("should format seconds less than a minute", () => {
    expect(formatUptime(30)).toBe("< 1m")
  })

  it("should format minutes", () => {
    expect(formatUptime(300)).toBe("5m")
  })

  it("should format hours and minutes", () => {
    expect(formatUptime(3720)).toBe("1h 2m")
  })

  it("should format days, hours, and minutes", () => {
    expect(formatUptime(90060)).toBe("1d 1h 1m")
  })

  it("should format exact hours", () => {
    expect(formatUptime(7200)).toBe("2h")
  })

  it("should format exact days", () => {
    expect(formatUptime(86400)).toBe("1d")
  })
})
