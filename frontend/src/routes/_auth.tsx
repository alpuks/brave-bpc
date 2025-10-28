import { useEffect } from "react";
import { createFileRoute, Outlet } from "@tanstack/react-router";
import { Spinner } from "@heroui/react";
import { useAuth } from "../contexts/AuthContext";

export const Route = createFileRoute("/_auth")({
  component: AuthLayout,
});

function AuthLayout() {
  const { isAuthenticated, isLoading } = useAuth();

  useEffect(() => {
    if (isLoading || isAuthenticated) {
      return;
    }

    const port = import.meta.env.VITE_BACKEND_PORT ?? "2727";
    const src = encodeURIComponent(window.location.href);
    const host = window.location.hostname;
    const proto = window.location.protocol;
    const href = `${proto}//${host}:${port}/login?src=${src}`;
    window.location.assign(href);
  }, [isAuthenticated, isLoading]);

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
