import { QueryCache, QueryClient } from "@tanstack/react-query";
import { authKey } from "@/features/auth/queries";
import { ApiError } from "@/lib/http";

export const queryClient: QueryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error) => {
      if (error instanceof ApiError && error.status === 401) void queryClient.invalidateQueries({ queryKey: authKey });
    },
  }),
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
});
