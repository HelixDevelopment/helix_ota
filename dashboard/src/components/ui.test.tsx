// Helix OTA dashboard — unit/component tests for the shared UI primitives.
// Anti-bluff (§11.4): each test renders the real component and asserts the
// user-visible DOM the operator would actually see/interact with.

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorPanel,
  Field,
  ProgressBar,
  Table,
  TextInput,
} from "./ui";
import { ApiError } from "../api/client";

describe("Button", () => {
  it("invokes onClick when clicked", () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Save</Button>);
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("does not fire onClick while disabled", () => {
    const onClick = vi.fn();
    render(
      <Button onClick={onClick} disabled>
        Save
      </Button>,
    );
    const btn = screen.getByRole("button", { name: "Save" });
    expect(btn).toBeDisabled();
    fireEvent.click(btn);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("defaults to type=button and honours an explicit submit type", () => {
    const { rerender } = render(<Button>Default</Button>);
    expect(screen.getByRole("button", { name: "Default" })).toHaveAttribute("type", "button");
    rerender(<Button type="submit">Go</Button>);
    expect(screen.getByRole("button", { name: "Go" })).toHaveAttribute("type", "submit");
  });
});

describe("Badge", () => {
  it("renders its children text", () => {
    render(<Badge tone="ok">active</Badge>);
    expect(screen.getByText("active")).toBeInTheDocument();
  });

  it("applies the ok-tone background colour", () => {
    render(<Badge tone="ok">ok</Badge>);
    // ok tone background is #dcfce7 -> rgb(220, 252, 231)
    expect(screen.getByText("ok")).toHaveStyle({ background: "rgb(220, 252, 231)" });
  });

  it("applies the err-tone background colour distinct from ok", () => {
    render(<Badge tone="err">err</Badge>);
    // err tone background is #fee2e2 -> rgb(254, 226, 226)
    expect(screen.getByText("err")).toHaveStyle({ background: "rgb(254, 226, 226)" });
  });

  it("defaults to the neutral tone when no tone is passed", () => {
    render(<Badge>plain</Badge>);
    // neutral background #eef1f5 -> rgb(238, 241, 245)
    expect(screen.getByText("plain")).toHaveStyle({ background: "rgb(238, 241, 245)" });
  });
});

describe("ProgressBar", () => {
  it("exposes the rounded percentage via aria-valuenow", () => {
    render(<ProgressBar value={25} max={100} />);
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "25");
  });

  it("clamps values above max to 100", () => {
    render(<ProgressBar value={250} max={100} />);
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "100");
  });

  it("clamps negative values to 0", () => {
    render(<ProgressBar value={-5} max={100} />);
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "0");
  });

  it("scales against a non-100 max", () => {
    // value 1 of max 4 -> 25%
    render(<ProgressBar value={1} max={4} />);
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "25");
  });

  it("renders 0 when max is non-positive (no divide-by-zero)", () => {
    render(<ProgressBar value={10} max={0} />);
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "0");
  });
});

describe("Table", () => {
  it("renders the head cells and the row body", () => {
    render(
      <Table head={["version", "status"]}>
        <tr>
          <td>1.0.0</td>
          <td>published</td>
        </tr>
      </Table>,
    );
    expect(screen.getByRole("columnheader", { name: "version" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "status" })).toBeInTheDocument();
    expect(screen.getByText("1.0.0")).toBeInTheDocument();
    expect(screen.getByText("published")).toBeInTheDocument();
  });
});

describe("Card", () => {
  it("renders the optional title and children", () => {
    render(
      <Card title="Recent releases">
        <p>body</p>
      </Card>,
    );
    expect(screen.getByRole("heading", { name: "Recent releases" })).toBeInTheDocument();
    expect(screen.getByText("body")).toBeInTheDocument();
  });

  it("omits the heading when no title is given", () => {
    render(
      <Card>
        <p>only body</p>
      </Card>,
    );
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
    expect(screen.getByText("only body")).toBeInTheDocument();
  });
});

describe("Field + TextInput", () => {
  it("associates the label and emits the raw value on change", () => {
    let last = "";
    render(
      <Field label="version">
        <TextInput value="" onChange={(v) => (last = v)} placeholder="1.0.0" />
      </Field>,
    );
    expect(screen.getByText("version")).toBeInTheDocument();
    const input = screen.getByPlaceholderText("1.0.0");
    fireEvent.change(input, { target: { value: "1.2.3" } });
    expect(last).toBe("1.2.3");
  });

  it("renders a password input type when requested", () => {
    render(<TextInput value="" onChange={() => {}} type="password" placeholder="pw" />);
    expect(screen.getByPlaceholderText("pw")).toHaveAttribute("type", "password");
  });
});

describe("ErrorPanel", () => {
  it("renders an ApiError's status, code, message and request id", () => {
    const err = new ApiError(409, "VERSION_NOT_MONOTONIC", "version must increase", "req-abc");
    render(<ErrorPanel error={err} />);
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("409 VERSION_NOT_MONOTONIC");
    expect(alert).toHaveTextContent("version must increase");
    expect(alert).toHaveTextContent("request_id: req-abc");
  });

  it("renders per-field detail messages from an ApiError", () => {
    const err = new ApiError(400, "VALIDATION_ERROR", "invalid", undefined, [
      { field: "version", message: "required" },
      { message: "generic problem" },
    ]);
    render(<ErrorPanel error={err} />);
    expect(screen.getByText("version: required")).toBeInTheDocument();
    expect(screen.getByText("generic problem")).toBeInTheDocument();
  });

  it("renders a plain Error's message", () => {
    render(<ErrorPanel error={new Error("network down")} />);
    expect(screen.getByRole("alert")).toHaveTextContent("network down");
  });

  it("stringifies a non-Error value", () => {
    render(<ErrorPanel error="raw string failure" />);
    expect(screen.getByRole("alert")).toHaveTextContent("raw string failure");
  });
});

describe("EmptyState", () => {
  it("renders its message", () => {
    render(<EmptyState>No releases yet.</EmptyState>);
    expect(screen.getByText("No releases yet.")).toBeInTheDocument();
  });
});
