import { HeroUIProvider } from "@heroui/react";
import { Outlet, createRootRoute } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import { AuthProvider } from "../contexts/AuthContext";

export const Route = createRootRoute({
  component: RootComponent,
});

function RootComponent() {
  return (
    <AuthProvider>
      <HeroUIProvider>
        <main className="dark text-foreground bg-background">
          <div>Hello "__root"!</div>
          <Outlet />
          <TanStackRouterDevtools />
        </main>
      </HeroUIProvider>
    </AuthProvider>
  );
}
