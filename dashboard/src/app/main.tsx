import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "@tanstack/react-router";
import "@/styles/global.css";
import { ToastProvider } from "@/components/Toasts/Toasts";
import { ErrorBoundary } from "@/components/PageQuery/ErrorBoundary";
import { router } from "@/router";
import { queryClient } from "./queryClient";
import { ThemeProvider } from "./theme";
import { UiProvider } from "./ui";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <ToastProvider>
          <UiProvider>
            <ErrorBoundary>
              <Suspense fallback={null}>
                <RouterProvider router={router} />
              </Suspense>
            </ErrorBoundary>
          </UiProvider>
        </ToastProvider>
      </ThemeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
