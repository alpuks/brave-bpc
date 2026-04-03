import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthLayout } from "../../routes/_auth";

const mocks = vi.hoisted(() => ({
  useAuth: vi.fn(),
  navigate: vi.fn(),
}));

vi.mock("../../contexts/AuthContext", () => ({
  useAuth: mocks.useAuth,
}));

vi.mock("@tanstack/react-router", () => ({
  Outlet: () => <div>Protected content</div>,
  createFileRoute: () => () => ({}),
  useRouter: () => ({
    navigate: mocks.navigate,
  }),
}));

describe("AuthLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows a loading state while session verification is running", () => {
    mocks.useAuth.mockReturnValue({
      isAuthenticated: false,
      isLoading: true,
    });

    render(<AuthLayout />);

    expect(screen.getByText("Verifying session...")).toBeInTheDocument();
    expect(mocks.navigate).not.toHaveBeenCalled();
  });

  it("redirects unauthenticated users to the home page", async () => {
    mocks.useAuth.mockReturnValue({
      isAuthenticated: false,
      isLoading: false,
    });

    render(<AuthLayout />);

    await waitFor(() => {
      expect(mocks.navigate).toHaveBeenCalledWith({ to: "/", replace: true });
    });
  });

  it("renders protected content for authenticated users", () => {
    mocks.useAuth.mockReturnValue({
      isAuthenticated: true,
      isLoading: false,
    });

    render(<AuthLayout />);

    expect(screen.getByText("Protected content")).toBeInTheDocument();
    expect(mocks.navigate).not.toHaveBeenCalled();
  });
});
