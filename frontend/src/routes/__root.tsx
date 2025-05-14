import { HeroUIProvider, Link } from "@heroui/react";
import { Outlet, createRootRouteWithContext  } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import type { AuthContext } from "../contexts/AuthContext";
import { NavBar } from "../components/NavBar";
import { useTheme } from "../contexts/ThemeContext";

interface MyRouterContext {
  auth: AuthContext
}

export const Route = createRootRouteWithContext<MyRouterContext>()({
  component: RootComponent,
  notFoundComponent: () => {
    return (
      <div className="flex gap-4">
        <p>Page not found</p>
        <Link showAnchorIcon href="/">Back Home</Link>
      </div>
    )
  },
});

function RootComponent() {
  const {auth} = Route.useRouteContext()
  const {theme} = useTheme()
  // TODO add light/dark mode toggle via adding dark to className
  return (

      <HeroUIProvider>
        <main className={`${theme} text-foreground bg-background`}>
          <NavBar authContext={auth}/>
          <Outlet />
          <TanStackRouterDevtools />
        </main>
      </HeroUIProvider>

  );
}
