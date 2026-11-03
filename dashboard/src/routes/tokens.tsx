import { createFileRoute } from "@tanstack/react-router";
import { prefetch } from "@/app/loaders";
import { tokensQuery } from "@/features/tokens/queries";
import { Tokens } from "@/pages/tokens/Tokens";

export const Route = createFileRoute("/tokens")({
  loader: prefetch(tokensQuery),
  component: Tokens,
});
