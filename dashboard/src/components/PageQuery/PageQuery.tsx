import type { UseQueryResult } from "@tanstack/react-query";
import type { ReactNode } from "react";

interface Props<T> {
  query: Pick<UseQueryResult<T>, "data">;
  empty?: ReactNode;
  children: (data: T) => ReactNode;
}

export function PageQuery<T>({ query, empty, children }: Props<T>) {
  const data = query.data as T;
  if (empty !== undefined && Array.isArray(data) && data.length === 0) {
    return <>{empty}</>;
  }
  return <>{children(data)}</>;
}
