import { render, screen } from "@testing-library/react"
import StatusCard from "../../src/components/StatusCard"

describe("StatusCard", () => {
  const status = {
    name: "Some Name Here",
    revision: "3.76.1",
    repo_url: "https://www.github.com/1234",
    ref_type: "branch",
    ref_value: "main",
    last_change_at: "2026-03-22T10:30:00Z",
    last_deployed_at: "2026-03-22T10:31:00Z"
  }

  it("should render name, revision, and repo_url properties", () => {
    render(
      <StatusCard
        name={status.name}
        error={""}
        revision={status.revision}
        repo_url={status.repo_url}
        ref_type={status.ref_type}
        ref_value={status.ref_value}
        last_change_at={status.last_change_at}
        last_deployed_at={status.last_deployed_at}
      />
    )

    expect(screen.getByText(status.name)).toBeInTheDocument()
    expect(screen.getByText(status.revision)).toBeInTheDocument()
    expect(screen.getByText(status.repo_url)).toBeInTheDocument()
  })

  it("should render ref_type:ref_value for watching field", () => {
    render(
      <StatusCard
        name={status.name}
        error={""}
        revision={status.revision}
        repo_url={status.repo_url}
        ref_type={status.ref_type}
        ref_value={status.ref_value}
        last_change_at={status.last_change_at}
        last_deployed_at={status.last_deployed_at}
      />
    )

    expect(screen.getByText("branch:main")).toBeInTheDocument()
  })

  it("should render repo_url as a link", () => {
    render(
      <StatusCard
        name={status.name}
        error={""}
        revision={status.revision}
        repo_url={status.repo_url}
        ref_type={status.ref_type}
        ref_value={status.ref_value}
        last_change_at={status.last_change_at}
        last_deployed_at={status.last_deployed_at}
      />
    )

    const repoUrlElement = screen.getByRole("link", { name: status.repo_url })
    expect(repoUrlElement).toBeInTheDocument()
    expect(repoUrlElement).toHaveAttribute("href", status.repo_url)
  })

  it("should not render error if it is empty", () => {
    render(
      <StatusCard
        name={status.name}
        error={""}
        revision={status.revision}
        repo_url={status.repo_url}
        ref_type={status.ref_type}
        ref_value={status.ref_value}
        last_change_at={status.last_change_at}
        last_deployed_at={status.last_deployed_at}
      />
    )

    const errorText = screen.queryByText(/error/i)
    expect(errorText).not.toBeInTheDocument()
  })

  it("should render error if it is not empty", () => {
    render(
      <StatusCard
        name={status.name}
        error={"Oh no!"}
        revision={status.revision}
        repo_url={status.repo_url}
        ref_type={status.ref_type}
        ref_value={status.ref_value}
        last_change_at={status.last_change_at}
        last_deployed_at={status.last_deployed_at}
      />
    )

    const errorText = screen.queryByText(/error/i)
    expect(errorText).toBeInTheDocument()
  })

  it("should render last_change_at timestamp", () => {
    render(
      <StatusCard
        name={status.name}
        error={""}
        revision={status.revision}
        repo_url={status.repo_url}
        ref_type={status.ref_type}
        ref_value={status.ref_value}
        last_change_at={status.last_change_at}
        last_deployed_at={status.last_deployed_at}
      />
    )

    expect(screen.getByText("Last Change:")).toBeInTheDocument()
  })

  it("should render last_deployed_at timestamp", () => {
    render(
      <StatusCard
        name={status.name}
        error={""}
        revision={status.revision}
        repo_url={status.repo_url}
        ref_type={status.ref_type}
        ref_value={status.ref_value}
        last_change_at={status.last_change_at}
        last_deployed_at={status.last_deployed_at}
      />
    )

    expect(screen.getByText("Last Deployed:")).toBeInTheDocument()
  })

  it("should render N/A for empty timestamp", () => {
    render(
      <StatusCard
        name={status.name}
        error={""}
        revision={status.revision}
        repo_url={status.repo_url}
        ref_type={status.ref_type}
        ref_value={status.ref_value}
        last_change_at={""}
        last_deployed_at={""}
      />
    )

    const naTexts = screen.getAllByText("N/A")
    expect(naTexts).toHaveLength(2)
  })
})
