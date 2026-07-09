import * as React from "react";
import {
  Controller,
  FormProvider,
  type Control,
  type ControllerRenderProps,
  type FieldValues,
  type FieldPath,
} from "react-hook-form";
import { cn } from "@/lib/utils";

// `Form` is react-hook-form's FormProvider so `<Form {...form}>` (the shadcn
// idiom) supplies the form context to every FormField below it. The actual
// <form> element is rendered by each consumer around its fields.
const Form = FormProvider;

// `FormField` wraps react-hook-form's Controller so the `field` render-prop
// carries the real ControllerRenderProps ({ value, onChange, onBlur, name, ref })
// instead of the previous stub that only exposed `name`.
function FormField<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(props: {
  control: Control<TFieldValues>;
  name: TName;
  render: (renderProps: { field: ControllerRenderProps<TFieldValues, TName> }) => React.ReactElement;
}) {
  return <Controller control={props.control} name={props.name} render={props.render} />;
}

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

const FormDescription = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(({ className, ...props }, ref) => (
  <p ref={ref} className={cn("text-sm text-muted-foreground", className)} {...props} />
));
FormDescription.displayName = "FormDescription";

function useFormField() {
  return { id: "", error: false };
}

export { Form, FormField, FormItem, FormLabel, FormControl, FormMessage, FormDescription, useFormField };
