import { fireEvent, render, screen } from "@testing-library/react"
import ConfigWarningBanner from "../../src/components/ConfigWarningBanner"

describe("ConfigWarningBanner", () => {
  it("should render warning text when warnings are present", () => {
    render(<ConfigWarningBanner warnings={["Something is misconfigured"]} />)

    expect(screen.getByText("Something is misconfigured")).toBeInTheDocument()
    expect(screen.getByText(/Configuration Warning/)).toBeInTheDocument()
  })

  it("should render multiple warnings", () => {
    render(<ConfigWarningBanner warnings={["Warning one", "Warning two"]} />)

    expect(screen.getByText("Warning one")).toBeInTheDocument()
    expect(screen.getByText("Warning two")).toBeInTheDocument()
    expect(screen.getByText(/Configuration Warnings/)).toBeInTheDocument()
  })

  it("should not render when warnings array is empty", () => {
    const { container } = render(<ConfigWarningBanner warnings={[]} />)

    expect(container.firstChild).toBeNull()
  })

  it("should be dismissible", () => {
    render(<ConfigWarningBanner warnings={["A warning message"]} />)

    expect(screen.getByText("A warning message")).toBeInTheDocument()

    const dismissButton = screen.getByRole("button", { name: /dismiss/i })
    fireEvent.click(dismissButton)

    expect(screen.queryByText("A warning message")).not.toBeInTheDocument()
  })
})
