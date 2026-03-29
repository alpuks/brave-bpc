import { useEffect } from "react";
import { createFileRoute, Outlet, useRouter } from "@tanstack/react-router";
import { Spinner } from "@heroui/react";
import { useAuth } from "../contexts/AuthContext";

export const Route = createFileRoute("/_auth")({
  component: AuthLayout,
});

export function AuthLayout() {
  const { isAuthenticated, isLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (isLoading || isAuthenticated) {
      return;
    }

    void router.navigate({ to: "/", replace: true });
  }, [isAuthenticated, isLoading, router]);

  if (isLoading) {
    return (
      <div className="flex h-full w-full items-center justify-center p-6">
        <Spinner label="Verifying session..." size="lg" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  return (
    <div className="p-2 h-full">
      <Outlet />
    </div>
  );
}
