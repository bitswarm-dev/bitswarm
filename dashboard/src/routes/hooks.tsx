import { createFileRoute } from "@tanstack/react-router";
import { prefetch } from "@/app/loaders";
import { deliveriesQuery, hooksConfigQuery, hooksQuery } from "@/features/hooks/queries";
import { Hooks } from "@/pages/hooks/Hooks";
import { HooksGate } from "@/pages/hooks/HooksGate";

export const Route = createFileRoute("/hooks")({
  loader: prefetch(hooksQuery, deliveriesQuery, hooksConfigQuery),
  component: () => (
    <HooksGate>
      <Hooks />
    </HooksGate>
  ),
});
