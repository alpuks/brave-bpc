"use client";

import { Button, addToast } from "@heroui/react";
import { createFileRoute } from "@tanstack/react-router";
import {
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { Selection, SortDescriptor } from "@react-types/shared";
import {
  useRequisitionsQuery,
  type BlueprintRequest,
} from "../api/requisitions";
import { useEsiNames } from "../api/esi";
import { useAuth } from "../contexts/AuthContext";
import RequestsTable from "../components/RequestsTable";
import BlueprintsTable from "../components/BlueprintsTable";

export const Route = createFileRoute("/_auth/requests")({
  component: RouteComponent,
});

type ChipColor =
  | "default"
  | "primary"
  | "secondary"
  | "success"
  | "warning"
  | "danger";

type StatusMetadataEntry = {
  value: number;
  slug: string;
  name: string;
  color: ChipColor;
};

const statusMetadata: Record<number, StatusMetadataEntry> = {
  0: { value: 0, slug: "open", name: "Open", color: "primary" },
  1: { value: 1, slug: "canceled", name: "Canceled", color: "default" },
  2: { value: 2, slug: "completed", name: "Completed", color: "success" },
  3: { value: 3, slug: "rejected", name: "Rejected", color: "danger" },
};

const OPEN_STATUS_VALUE = 0;

const statusMetadataBySlug: Record<string, StatusMetadataEntry> = {};
for (const entry of Object.values(statusMetadata)) {
  statusMetadataBySlug[entry.slug] = entry;
}

const statusFilterOptions = Object.values(statusMetadata).map((entry) => ({
  value: entry.value,
  label: entry.name,
}));

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

function resolveStatusMetadata(status?: string | number): StatusMetadataEntry {
  if (status == null) {
    return statusMetadata[OPEN_STATUS_VALUE];
  }

  if (typeof status === "number" && Number.isInteger(status)) {
    return statusMetadata[status] ?? statusMetadata[OPEN_STATUS_VALUE];
  }

  const numericStatus = Number(status);
  if (Number.isInteger(numericStatus)) {
    const byNumber = statusMetadata[numericStatus];
    if (byNumber) {
      return byNumber;
    }
  }

  if (typeof status === "string") {
    const bySlug = statusMetadataBySlug[status.toLowerCase()];
    if (bySlug) {
      return bySlug;
    }
  }

  return statusMetadata[OPEN_STATUS_VALUE];
}

function isOpenStatus(status?: string | number): boolean {
  return resolveStatusMetadata(status).value === OPEN_STATUS_VALUE;
}

function RouteComponent() {
  const { user } = useAuth();

  if (!user) {
    throw new Error("Auth guard did not provide a user");
  }

  const { auth_level, character_id } = user;
  const [statusFilter, setStatusFilter] = useState<number>(0);
  const [characterFilter, setCharacterFilter] = useState<string>("");

  const {
    data: requisitions = [],
    isLoading,
    error,
    refetch,
  } = useRequisitionsQuery(statusFilter);

  const [selectedKey, setSelectedKey] = useState<number | null>(null);
  const [sortDescriptor, setSortDescriptor] = useState<SortDescriptor>({
    column: "updated_at",
    direction: "descending",
  });

  const requiresLocking = auth_level >= LOCK_AUTH_THRESHOLD;
  const selectingRef = useRef(false);

  const requestById = useMemo(() => {
    const map = new Map<number, BlueprintRequest>();
    for (const request of requisitions) {
      map.set(request.id, request);
    }
    return map;
  }, [requisitions]);

  const filteredRequisitions = useMemo(() => {
    const query = characterFilter.trim().toLowerCase();
    if (query.length === 0) {
      return requisitions;
    }

    return requisitions.filter((req) => {
      const name = req.character_name?.toLowerCase() ?? "";
      return name.includes(query);
    });
  }, [characterFilter, requisitions]);

  const sortedItems = useMemo<BlueprintRequest[]>(() => {
    const items = [...filteredRequisitions];
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
          return resolveStatusMetadata(item.status).value;
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
  }, [filteredRequisitions, sortDescriptor]);

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

  const formatDate = useCallback((iso: string) => {
    const ts = Date.parse(iso);
    return Number.isFinite(ts) ? dateTimeFormatter.format(ts) : "";
  }, []);

  const shouldLockRequest = useCallback(
    (request?: BlueprintRequest | null) => {
      if (!requiresLocking || !request) {
        return false;
      }
      return isOpenStatus(request.status);
    },
    [requiresLocking]
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

      const request = requestById.get(key);
      const lockable = shouldLockRequest(request);
      const currentRequest =
        selectedKey === null ? null : (requestById.get(selectedKey) ?? null);
      const currentLockable = shouldLockRequest(currentRequest);

      try {
        if (selectedKey === key) {
          if (lockable) {
            await releaseLock(key).catch(() => undefined);
          }
          setSelectedKey(null);
          return;
        }

        if (currentLockable && selectedKey !== null && selectedKey !== key) {
          return;
        }

        if (lockable) {
          await acquireLock(key).catch(() => undefined);
        }
        setSelectedKey(key);
      } finally {
        selectingRef.current = false;
      }
    },
    [selectedKey, requestById, shouldLockRequest, acquireLock, releaseLock]
  );

  const selectedRequest = useMemo(() => {
    if (selectedKey === null) {
      return null;
    }
    return requestById.get(selectedKey) ?? null;
  }, [requestById, selectedKey]);

  const selectedKeys = useMemo<Selection>(() => {
    return selectedKey === null
      ? new Set<string>()
      : new Set<string>([String(selectedKey)]);
  }, [selectedKey]);

  const handleSelectionChange = useCallback(
    (keys: Selection | "all") => {
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

      void toggleExpand(nextKey);
    },
    [selectedKey, toggleExpand]
  );

  const handleView = useCallback(
    (id: number) => {
      if (selectingRef.current) return;
      const currentRequest =
        selectedKey === null ? null : (requestById.get(selectedKey) ?? null);
      if (
        currentRequest &&
        selectedKey !== id &&
        shouldLockRequest(currentRequest)
      ) {
        return;
      }
      void toggleExpand(id);
    },
    [requestById, selectedKey, shouldLockRequest, toggleExpand]
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
          title: action.charAt(0).toUpperCase() + action.slice(1),
          description: `Successfully updated request ${requestId}`,
          color: "success",
        });
      } catch (err) {
        addToast({
          title: `Error`,
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

  useEffect(() => {
    setSelectedKey((current) => {
      if (current === null) {
        return current;
      }

      const currentRequest = requestById.get(current);
      if (shouldLockRequest(currentRequest)) {
        void releaseLock(current).catch(() => undefined);
      }

      return null;
    });
  }, [
    statusFilter,
    characterFilter,
    requestById,
    shouldLockRequest,
    releaseLock,
  ]);

  return (
    <div className="flex w-full flex-row gap-4">
      <div className="flex w-full flex-col gap-4">
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-2">
            <label
              className="text-sm font-medium text-default-600"
              htmlFor="status-filter"
            >
              Status
            </label>
            <select
              id="status-filter"
              className="rounded-medium border border-default-300 bg-content1 px-3 py-2 text-sm text-default-600"
              value={String(statusFilter)}
              onChange={(event) => setStatusFilter(Number(event.target.value))}
            >
              {statusFilterOptions.map((option) => (
                <option key={option.value} value={String(option.value)}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>

          <div className="flex items-center gap-2">
            <label
              className="text-sm font-medium text-default-600"
              htmlFor="character-filter"
            >
              Character
            </label>
            <input
              id="character-filter"
              autoComplete="off"
              className="rounded-medium border border-default-300 bg-content1 px-3 py-2 text-sm text-default-600"
              onChange={(event) => setCharacterFilter(event.target.value)}
              placeholder="Filter by character"
              type="text"
              value={characterFilter}
            />
          </div>
        </div>

        <RequestsTable
          error={error}
          isLoading={isLoading || areNamesLoading}
          items={sortedItems}
          selectedKey={selectedKey}
          selectedRequest={selectedRequest}
          shouldLockRequest={shouldLockRequest}
          resolveStatusMetadata={resolveStatusMetadata}
          formatDate={formatDate}
          onView={handleView}
          onSelectionChange={handleSelectionChange}
          onSortChange={setSortDescriptor}
          selectedKeys={selectedKeys}
          sortDescriptor={sortDescriptor}
        />
      </div>

      {selectedRequest && (
        <div className="rounded-medium border p-4">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-medium font-semibold">
              Request #{selectedRequest.id} details
            </h3>
            <div className="flex gap-2">
              {selectedRequest.character_id === character_id &&
                isOpenStatus(selectedRequest.status) && (
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
              {auth_level >= LOCK_AUTH_THRESHOLD &&
                isOpenStatus(selectedRequest.status) && (
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

          <BlueprintsTable
            blueprints={selectedRequest.blueprints}
            getNameById={getNameById}
          />
        </div>
      )}
    </div>
  );
}

export default RouteComponent;
