"use client";

import {
  Table,
  TableHeader,
  TableRow,
  TableBody,
  TableCell,
  TableColumn,
  Chip,
  Spinner,
  User,
  Snippet,
  Button,
  addToast,
} from "@heroui/react";
import { createFileRoute } from "@tanstack/react-router";
import { Fragment, useCallback, useMemo, useRef, useState } from "react";
import type { Selection, SortDescriptor } from "@react-types/shared";
import {
  useRequisitionsQuery,
  type BlueprintRequest,
} from "../api/requisitions";
import { useEsiNames } from "../api/esi";
import { useAuth } from "../contexts/AuthContext";

export const Route = createFileRoute("/_auth/requests")({
  component: RouteComponent,
});

const statusColorMap = {
  open: "primary",
  closed: "default",
  completed: "success",
  rejected: "danger",
} as const;

type Status = keyof typeof statusColorMap;

type RequisitionSortKey =
  | "id"
  | "character_id"
  | "status"
  | "created_at"
  | "updated_at"
  | "updated_by"
  | "public_notes";

const dateTimeFormatter = new Intl.DateTimeFormat(undefined, {
  year: "numeric",
  month: "short",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

const LOCK_AUTH_THRESHOLD = 2;

function RouteComponent() {
  const { user } = useAuth();

  if (!user) {
    throw new Error("Auth guard did not provide a user");
  }

  const { auth_level, character_id } = user;

  const {
    data: requisitions = [],
    isLoading,
    error,
    refetch,
  } = useRequisitionsQuery();

  const [selectedKey, setSelectedKey] = useState<number | null>(null);
  const [sortDescriptor, setSortDescriptor] = useState<SortDescriptor>({
    column: "updated_at",
    direction: "descending",
  });

  const requiresLocking = auth_level >= LOCK_AUTH_THRESHOLD;
  const selectingRef = useRef(false);

  const sortedItems = useMemo<BlueprintRequest[]>(() => {
    const items = [...requisitions];
    const column =
      typeof sortDescriptor.column === "string"
        ? (sortDescriptor.column as RequisitionSortKey)
        : undefined;

    if (!column) {
      return items;
    }

    const collator = new Intl.Collator(undefined, {
      numeric: true,
      sensitivity: "base",
    });

    const getValue = (item: BlueprintRequest) => {
      switch (column) {
        case "created_at":
        case "updated_at": {
          const timestamp = Date.parse(item[column]);
          return Number.isFinite(timestamp) ? timestamp : 0;
        }
        case "id":
        case "character_id":
          return item[column];
        case "status":
        case "updated_by":
        case "public_notes":
          return item[column] ?? "";
        default:
          return "";
      }
    };

    items.sort((a, b) => {
      const va = getValue(a);
      const vb = getValue(b);
      const cmp =
        typeof va === "number" && typeof vb === "number"
          ? va - vb
          : collator.compare(String(va), String(vb));
      return sortDescriptor.direction === "descending" ? -cmp : cmp;
    });

    return items;
  }, [requisitions, sortDescriptor]);

  const blueprintTypeIds = useMemo(() => {
    const ids = new Set<number>();
    for (const request of requisitions) {
      for (const bp of request.blueprints) {
        ids.add(bp.type_id);
      }
    }
    return Array.from(ids);
  }, [requisitions]);

  const { names, isLoading: areNamesLoading } = useEsiNames(blueprintTypeIds);

  const getNameById = useCallback(
    (id: number, fallback?: string) => names.get(id) ?? fallback ?? "Unknown",
    [names]
  );

  const acquireLock = useCallback(
    async (id: number) => {
      if (!requiresLocking) return;
      try {
        const response = await fetch(`/api/requisition/${id}/lock`, {
          method: "PATCH",
          credentials: "include",
        });
        if (!response.ok) {
          throw new Error(`Failed to acquire lock (${response.status})`);
        }
      } catch (err) {
        addToast({
          title: "Lock Error",
          description: `Failed to acquire lock for request ${id}: ${String(
            err
          )}`,
          color: "danger",
        });
        throw err;
      }
    },
    [requiresLocking]
  );

  const releaseLock = useCallback(
    async (id: number) => {
      if (!requiresLocking) return;
      try {
        const response = await fetch(`/api/requisition/${id}/unlock`, {
          method: "PATCH",
          credentials: "include",
        });
        if (!response.ok) {
          throw new Error(`Failed to release lock (${response.status})`);
        }
      } catch (err) {
        addToast({
          title: "Lock Error",
          description: `Failed to release lock for request ${id}: ${String(
            err
          )}`,
          color: "danger",
        });
        throw err;
      }
    },
    [requiresLocking]
  );

  const toggleExpand = useCallback(
    async (key: number) => {
      if (selectingRef.current) return;
      selectingRef.current = true;

      try {
        if (selectedKey === key) {
          if (requiresLocking) {
            await releaseLock(key).catch(() => undefined);
          }
          setSelectedKey(null);
          return;
        }

        if (requiresLocking && selectedKey !== null) {
          return;
        }

        if (requiresLocking) {
          await acquireLock(key).catch(() => undefined);
        }
        setSelectedKey(key);
      } finally {
        selectingRef.current = false;
      }
    },
    [selectedKey, requiresLocking, acquireLock, releaseLock]
  );

  const selectedRequest = useMemo(
    () => requisitions.find((req) => req.id === selectedKey) ?? null,
    [requisitions, selectedKey]
  );

  const selectedKeys = useMemo<Selection>(() => {
    return selectedKey === null
      ? new Set<string>()
      : new Set<string>([String(selectedKey)]);
  }, [selectedKey]);

  const handleSelectionChange = useCallback(
    (keys: Selection) => {
      if (selectingRef.current) return;
      if (keys === "all") return;
      if (!(keys instanceof Set)) return;

      const first = keys.values().next();
      const rawValue = first.done ? null : first.value;
      const nextKey = rawValue === null ? null : Number(rawValue);

      if (nextKey === null || Number.isNaN(nextKey)) {
        if (selectedKey !== null) {
          void toggleExpand(selectedKey);
        }
        return;
      }

      if (!requiresLocking) {
        if (selectedKey !== nextKey) {
          void toggleExpand(nextKey);
        }
        return;
      }

      if (selectedKey === null || selectedKey === nextKey) {
        void toggleExpand(nextKey);
      }
    },
    [requiresLocking, selectedKey, toggleExpand]
  );

  const handleView = useCallback(
    (id: number) => {
      if (selectingRef.current) return;
      if (requiresLocking && selectedKey !== null && selectedKey !== id) return;
      void toggleExpand(id);
    },
    [requiresLocking, selectedKey, toggleExpand]
  );

  const handleRequestAction = useCallback(
    async (action: "cancel" | "complete" | "reject", requestId: number) => {
      try {
        const response = await fetch(
          `/api/requisition/${requestId}/${action}`,
          {
            method: "PATCH",
            credentials: "include",
          }
        );
        if (!response.ok) {
          throw new Error(`Failed to ${action} request (${response.status})`);
        }

        addToast({
          title:
            action === "cancel"
              ? "Cancelled"
              : action === "complete"
                ? "Completed"
                : "Rejected",
          description: `Successfully ${action}ed request ${requestId}`,
          color:
            action === "complete"
              ? "success"
              : action === "cancel"
                ? "warning"
                : "danger",
        });
      } catch (err) {
        addToast({
          title: `${action.charAt(0).toUpperCase()}${action.slice(1)} error`,
          description: `Failed to ${action} request ${requestId}: ${String(err)}`,
          color: "danger",
        });
        return;
      }

      await refetch();
      if (requiresLocking && selectedKey === requestId) {
        setSelectedKey(null);
      }
    },
    [refetch, requiresLocking, selectedKey]
  );

  const renderStatus = (status?: string) => (
    <Chip
      className="capitalize gap-1 text-default-600 text-bold"
      color={statusColorMap[(status as Status) ?? "open"]}
      radius="sm"
      size="lg"
      variant="bordered"
    >
      {status || "Open"}
    </Chip>
  );

  const renderDate = (iso: string) => {
    const ts = Date.parse(iso);
    return (
      <span className="text-default-500">
        {Number.isFinite(ts) ? dateTimeFormatter.format(ts) : ""}
      </span>
    );
  };

  const renderExpandButton = (id: number) => {
    const isSelected = selectedKey === id;

    if (!requiresLocking) {
      return (
        <Button
          disabled={isSelected}
          onPress={() => handleView(id)}
          size="sm"
          variant="flat"
        >
          {isSelected ? "Viewing" : "View"}
        </Button>
      );
    }

    const somethingSelected = selectedKey !== null;

    if (!somethingSelected) {
      return (
        <Button onPress={() => handleView(id)} size="sm" variant="flat">
          View
        </Button>
      );
    }

    if (isSelected) {
      return (
        <Button disabled size="sm" variant="flat">
          Viewing
        </Button>
      );
    }

    return (
      <Button disabled size="sm" variant="flat">
        View
      </Button>
    );
  };

  return (
    <div className="flex w-full flex-row gap-4">
      <Table
        aria-label="Requests Table"
        className="w-full"
        isStriped
        isVirtualized
        onSelectionChange={handleSelectionChange}
        onSortChange={setSortDescriptor}
        selectedKeys={selectedKeys}
        selectionMode="single"
        sortDescriptor={sortDescriptor}
      >
        <TableHeader>
          <TableColumn allowsSorting key="id">
            ID
          </TableColumn>
          <TableColumn key="requester">Requester</TableColumn>
          <TableColumn allowsSorting key="status">
            Status
          </TableColumn>
          <TableColumn allowsSorting key="created_at">
            Created At
          </TableColumn>
          <TableColumn allowsSorting key="updated_at">
            Updated At
          </TableColumn>
          <TableColumn key="updated_by">Updated By</TableColumn>
          <TableColumn key="public_notes">Public Notes</TableColumn>
          <TableColumn key="actions">Actions</TableColumn>
        </TableHeader>

        <TableBody
          emptyContent={error ? "Failed to load" : "No requests"}
          isLoading={isLoading || areNamesLoading}
          items={sortedItems}
          loadingContent={<Spinner label="Loading..." />}
        >
          {(item: BlueprintRequest) => {
            const characterName = item.character_name || "Unknown";
            return (
              <TableRow key={String(item.id)}>
                <TableCell>{item.id}</TableCell>
                <TableCell>
                  <User
                    avatarProps={{
                      src: `https://images.evetech.net/characters/${item.character_id}/portrait`,
                    }}
                    className="text-default-600"
                    id={item.character_id.toString()}
                    name={
                      <Snippet
                        hideSymbol
                        key={`char-${item.character_id}-${characterName}`}
                        radius="none"
                        size="sm"
                      >
                        {characterName}
                      </Snippet>
                    }
                  />
                </TableCell>
                <TableCell>{renderStatus(item.status)}</TableCell>
                <TableCell>{renderDate(item.created_at)}</TableCell>
                <TableCell>{renderDate(item.updated_at)}</TableCell>
                <TableCell>{item.updated_by || "N/A"}</TableCell>
                <TableCell>{item.public_notes}</TableCell>
                <TableCell>{renderExpandButton(item.id)}</TableCell>
              </TableRow>
            );
          }}
        </TableBody>
      </Table>

      {selectedRequest && (
        <div className="rounded-medium border p-4">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-medium font-semibold">
              Request #{selectedRequest.id} details
            </h3>
            <div className="flex gap-2">
              {selectedRequest.character_id === character_id && (
                <Button
                  color="warning"
                  onPress={() =>
                    handleRequestAction("cancel", selectedRequest.id)
                  }
                  variant="ghost"
                >
                  Cancel
                </Button>
              )}
              {auth_level >= LOCK_AUTH_THRESHOLD && (
                <Fragment>
                  <Button
                    color="success"
                    onPress={() =>
                      handleRequestAction("complete", selectedRequest.id)
                    }
                    variant="ghost"
                  >
                    Complete
                  </Button>
                  <Button
                    color="danger"
                    onPress={() =>
                      handleRequestAction("reject", selectedRequest.id)
                    }
                    variant="ghost"
                  >
                    Reject
                  </Button>
                </Fragment>
              )}
              <Button
                onPress={() => void toggleExpand(selectedRequest.id)}
                variant="flat"
              >
                Close
              </Button>
            </div>
          </div>

          <Table aria-label="Blueprints">
            <TableHeader>
              <TableColumn key="bp">Blueprint</TableColumn>
              <TableColumn key="me">ME</TableColumn>
              <TableColumn key="te">TE</TableColumn>
              <TableColumn key="runs">Runs</TableColumn>
              <TableColumn key="qty">Quantity</TableColumn>
            </TableHeader>
            <TableBody emptyContent="No blueprints">
              {selectedRequest.blueprints.map((blueprint) => (
                <TableRow key={blueprint.type_id}>
                  <TableCell>
                    <User
                      avatarProps={{
                        src: `https://images.evetech.net/types/${blueprint.type_id}/bpc`,
                      }}
                      name={getNameById(blueprint.type_id, blueprint.type_name)}
                    />
                  </TableCell>
                  <TableCell>{blueprint.material_efficiency ?? 0}</TableCell>
                  <TableCell>{blueprint.time_efficiency ?? 0}</TableCell>
                  <TableCell>{blueprint.runs}</TableCell>
                  <TableCell>{blueprint.quantity ?? 1}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}

export default RouteComponent;
