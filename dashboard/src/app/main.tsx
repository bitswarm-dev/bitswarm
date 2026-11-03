import { QueryCache, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "@tanstack/react-router";
import "@/styles/global.css";
import { ToastProvider } from "@/components/Toasts/Toasts";
import { authKey } from "@/features/auth/queries";
import { ApiError } from "@/lib/http";
import { ErrorBoundary } from "@/components/PageQuery/ErrorBoundary";
import { router } from "./router";
import { ThemeProvider } from "./theme";
import { UiProvider } from "./ui";

const queryClient: QueryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error) => {
      if (error instanceof ApiError && error.status === 401) void queryClient.invalidateQueries({ queryKey: authKey });
    },
  }),
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
});

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
