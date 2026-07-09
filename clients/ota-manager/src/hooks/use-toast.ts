import { useToast as useToastContext, type ToastProps } from "@/components/ui/toast";

export type ToastOptions = Omit<ToastProps, "id" | "onClose">;

// Adapter over the ToastProvider context. The context exposes
// `{ toasts, addToast, removeToast }`; call sites use the ergonomic
// `const { toast } = useToast(); toast({ title, description, variant })`
// shape, so this shim exposes `toast` (mapped onto `addToast`) alongside
// the raw context.
export function useToast() {
  const ctx = useToastContext();
  return {
    ...ctx,
    toast: (options: ToastOptions) => ctx.addToast(options),
  };
}
