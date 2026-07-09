import * as React from "react";
import { cn } from "@/lib/utils";

const Form = React.forwardRef<HTMLFormElement, React.FormHTMLAttributes<HTMLFormElement>>(({ className, ...props }, ref) => (
  <form ref={ref} className={cn("space-y-4", className)} {...props} />
));
Form.displayName = "Form";

const FormField = <T extends { name: string }>(props: { control?: unknown; name: string; render: (field: { field: T }) => React.ReactNode }) => {
  return <>{props.render({ field: { name: props.name } as T })}</>;
};

const FormItem = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(({ className, ...props }, ref) => (
  <div ref={ref} className={cn("space-y-1", className)} {...props} />
));
FormItem.displayName = "FormItem";

const FormLabel = React.forwardRef<HTMLLabelElement, React.LabelHTMLAttributes<HTMLLabelElement>>(({ className, ...props }, ref) => (
  <label ref={ref} className={cn("text-sm font-medium leading-none", className)} {...props} />
));
FormLabel.displayName = "FormLabel";

const FormControl = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>((props, ref) => <div ref={ref} {...props} />);
FormControl.displayName = "FormControl";

const FormMessage = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(({ className, children, ...props }, ref) => (
  children ? <p ref={ref} className={cn("text-sm text-destructive", className)} {...props}>{children}</p> : null
));
FormMessage.displayName = "FormMessage";

export { Form, FormField, FormItem, FormLabel, FormControl, FormMessage, FormDescription, useFormField };
function useFormField() {
  return { id: "", error: false };
}

const FormDescription = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(({ className, ...props }, ref) => (
  <p ref={ref} className={cn("text-sm text-muted-foreground", className)} {...props} />
));
FormDescription.displayName = "FormDescription";
