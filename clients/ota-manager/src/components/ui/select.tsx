import * as React from "react";
import { cn } from "@/lib/utils";

interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  onValueChange?: (value: string) => void;
}

const Select = React.forwardRef<HTMLSelectElement, SelectProps>(({ className, children, onValueChange, onChange, ...props }, ref) => (
  <select
    ref={ref}
    className={cn("flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring", className)}
    onChange={(e) => { onChange?.(e); onValueChange?.(e.target.value); }}
    {...props}
  >
    {children}
  </select>
));
Select.displayName = "Select";

const SelectTrigger = React.forwardRef<HTMLButtonElement, React.ButtonHTMLAttributes<HTMLButtonElement>>(({ className, children, ...props }, ref) => (
  <button ref={ref} className={cn("flex h-9 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm", className)} {...props}>
    {children}
  </button>
));
SelectTrigger.displayName = "SelectTrigger";

interface SelectValueProps extends React.HTMLAttributes<HTMLSpanElement> {
  placeholder?: string;
}

const SelectValue = React.forwardRef<HTMLSpanElement, SelectValueProps>(({ className, placeholder, children, ...props }, ref) => (
  <span ref={ref} className={cn("text-sm", className)} {...props}>
    {children ?? placeholder}
  </span>
));
SelectValue.displayName = "SelectValue";

const SelectContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(({ className, ...props }, ref) => (
  <div ref={ref} className={cn("relative z-50 max-h-60 overflow-auto rounded-md border bg-background shadow-lg", className)} {...props} />
));
SelectContent.displayName = "SelectContent";

const SelectItem = React.forwardRef<HTMLOptionElement, React.OptionHTMLAttributes<HTMLOptionElement>>(({ className, ...props }, ref) => (
  <option ref={ref} className={cn("relative flex cursor-default select-none items-center rounded-sm px-2 py-1 text-sm", className)} {...props} />
));
SelectItem.displayName = "SelectItem";

export { Select, SelectTrigger, SelectValue, SelectContent, SelectItem };
