import { StrictMode } from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider, createRouter,ErrorComponent } from "@tanstack/react-router";

import { routeTree } from "./routeTree.gen";

import "./index.css";
import { Spinner } from "@heroui/react";

const router = createRouter({
  routeTree,
  defaultPendingComponent: () => (
    <div className={`p-2 text-2xl`}>
      <Spinner />
    </div>
  ),
  defaultErrorComponent: ({ error }) => <ErrorComponent error={error} />,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

const rootElement = document.getElementById("root")!;
if (!rootElement.innerHTML) {
  const root = ReactDOM.createRoot(rootElement);
  root.render(
    <StrictMode>
      <RouterProvider router={router} />
    </StrictMode>
  );
}
