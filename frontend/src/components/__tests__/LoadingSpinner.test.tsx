import { render, screen } from "@testing-library/react";
import { LoadingSpinner } from "../LoadingSpinner";

describe("LoadingSpinner", () => {
  it("renders the message text", () => {
    render(<LoadingSpinner message="Loading data..." />);
    expect(screen.getByText("Loading data...")).toBeInTheDocument();
  });

  it("renders an SVG spinner", () => {
    const { container } = render(<LoadingSpinner message="Loading" />);
    expect(container.querySelector("svg")).toBeInTheDocument();
  });
});
