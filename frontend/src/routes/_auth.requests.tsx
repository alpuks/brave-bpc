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
import {
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { AsyncListData, useAsyncList } from "@react-stately/data";
import type { Selection } from "@react-types/shared";

export const Route = createFileRoute("/_auth/requests")({
  component: RouteComponent,
});

interface BlueprintRequest {
  id: number;
  character_id: number;
  character_name: string;
  status?: string;
  created_at: string;
  updated_at: string;
  updated_by?: string;
  public_notes?: string;
  blueprints: Array<{
    type_id: number;
    runs: number;
    material_efficiency?: number;
    time_efficiency?: number;
    quantity?: number;
    type_name?: string;
  }>;
}
interface EsiResponse {
  category: string;
  id: number;
  name: string;
}

const statusColorMap = {
  open: "primary",
  closed: "default",
  completed: "success",
  rejected: "danger",
} as const;
type Status = keyof typeof statusColorMap;

const dateTimeFormatter = new Intl.DateTimeFormat(undefined, {
  year: "numeric",
  month: "short",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

const LOCK_AUTH_THRESHOLD = 2; // TODO move to config

function RouteComponent() {
  const {
    auth: {
      // @ts-expect-error auth is checked above level in route
      user: { auth_level, character_id },
    },
  } = Route.useRouteContext();

  const [selectedKey, setSelectedKey] = useState<number | null>(null);
  const [idMap, setIdMap] = useState<Map<number, string>>(new Map());
  const requiresLocking = auth_level >= LOCK_AUTH_THRESHOLD;

  const acquireLock = useCallback(
    async (id: number) => {
      if (!requiresLocking) return null;
      console.debug(`acquireLock: requesting lock for ${id}`);
      try {
        const response = await fetch(`/api/requisition/${id}/lock`, {
          method: "PATCH",
          headers: {
            "Content-Type": "application/json",
          },
        });
        if (!response.ok) {
          throw new Error(`Failed to acquire lock (${response.status})`);
        }
      } catch (err) {
        addToast({
          title: "Lock Error",
          description: `Failed to acquire lock for request ${id}: ${String(err)}`,
          color: "danger",
        });
        return;
      }

      console.debug(`acquireLock: lock acquired for ${id}`);
      return {
        owner: "you",
        expires_at: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      };
    },
    [requiresLocking]
  );

  const releaseLock = useCallback(
    async (id: number) => {
      if (!requiresLocking) return;
      console.log(`releaseLock: releasing lock for ${id}`);
      try {
        const response = await fetch(`/api/requisition/${id}/unlock`, {
          method: "PATCH",
          headers: {
            "Content-Type": "application/json",
          },
        });
        if (!response.ok) {
          throw new Error(`Failed to release lock (${response.status})`);
        }
      } catch (err) {
        addToast({
          title: "Lock Error",
          description: `Failed to release lock for request ${id}: ${String(err)}`,
          color: "danger",
        });
        return;
      }

      console.log(`releaseLock: lock released for ${id}`);
    },
    [requiresLocking]
  );

  const selectingRef = useRef(false);

  const toggleExpand = useCallback(
    async (key: number) => {
      if (selectingRef.current) return;
      selectingRef.current = true;
      try {
        if (selectedKey === key) {
          if (requiresLocking) {
            await releaseLock(key).catch(() => {});
          }
          setSelectedKey(null);
          return;
        }

        if (requiresLocking && selectedKey !== null) {
          return;
        }

        if (requiresLocking) {
          try {
            await acquireLock(key);
          } catch (err) {
            console.warn("Lock failed, opening read-only", err);
          }
        }
        setSelectedKey(key);
      } finally {
        selectingRef.current = false;
      }
    },
    [selectedKey, requiresLocking, acquireLock, releaseLock]
  );

  const asyncList: AsyncListData<BlueprintRequest> = useAsyncList({
    load: async ({ signal }) => {
      const response = await fetch("/api/requisition", { signal });

      if (!response.ok) throw new Error(`Failed to load (${response.status})`);
      const data = await response.json();

      return { items: data };
    },
    sort: async ({ items, sortDescriptor }) => {
      const { direction } = sortDescriptor;
      const column =
        typeof sortDescriptor.column === "string"
          ? sortDescriptor.column
          : undefined;

      if (!column) return { items };

      const collator = new Intl.Collator(undefined, {
        numeric: true,
        sensitivity: "base",
      });

      const getValue = (item: BlueprintRequest) => {
        switch (column) {
          case "created_at": {
            const timestamp = Date.parse(item.created_at);
            return Number.isFinite(timestamp) ? timestamp : 0;
          }
          case "updated_at": {
            const timestamp = Date.parse(item.updated_at);
            return Number.isFinite(timestamp) ? timestamp : 0;
          }
          case "id":
            return item.id;
          case "character_id":
            return item.character_id;
          case "status":
            return item.status ?? "";
          case "updated_by":
            return item.updated_by ?? "";
          case "public_notes":
            return item.public_notes ?? "";
          default:
            return "";
        }
      };

      const sorted = [...items].sort((a, b) => {
        const va = getValue(a);
        const vb = getValue(b);
        const cmp =
          typeof va === "number" && typeof vb === "number"
            ? va - vb
            : collator.compare(String(va), String(vb));
        return direction === "descending" ? -cmp : cmp;
      });

      return { items: sorted };
    },
  });

  const neededIds = useMemo(() => {
    const ids = new Set<number>();
    for (const item of asyncList.items) {
      ids.add(item.character_id);
      for (const bp of item.blueprints) ids.add(bp.type_id);
    }
    return ids;
  }, [asyncList.items]);

  useEffect(() => {
    const missing = Array.from(neededIds).filter((id) => !idMap.has(id));
    if (!missing.length) return;

    const ac = new AbortController();
    const chunkSize = 1000;
    const chunks: number[][] = [];
    for (let i = 0; i < missing.length; i += chunkSize) {
      chunks.push(missing.slice(i, i + chunkSize));
    }

    (async () => {
      const results = await Promise.all(
        chunks.map((body) =>
          fetch("https://esi.evetech.net/universe/names", {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              Accept: "application/json",
            },
            body: JSON.stringify(body),
            signal: ac.signal,
          }).then((r) => {
            if (!r.ok) throw new Error(`names lookup failed (${r.status})`);
            return r.json() as Promise<EsiResponse[]>;
          })
        )
      );

      setIdMap((prev) => {
        const next = new Map(prev);
        for (const res of results.flat()) {
          next.set(res.id, res.name);
        }
        return next;
      });
    })().catch(() => {
      if (ac.signal.aborted) return;
    });

    return () => ac.abort();
  }, [neededIds, idMap]);

  const getNameById = useCallback(
    (id: number) => idMap.get(id) ?? "Unknown",
    [idMap]
  );

  const selectedRequest = useMemo(
    () => asyncList.items.find((i) => i.id === selectedKey) ?? null,
    [asyncList.items, selectedKey]
  );

  // Always clone the array so virtualized table rows receive new object references.
  const tableItems = asyncList.items.map((item) => ({ ...item }));

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
        void toggleExpand(nextKey);
        return;
      }

      if (selectedKey === null || selectedKey === nextKey) {
        void toggleExpand(nextKey);
      }
      // otherwise (different row while locked) ignore
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

  const renderStatus = (status?: string) => (
    <Chip
      className="capitalize gap-1 text-default-600 text-bold"
      color={statusColorMap[(status as Status) ?? "open"]}
      size="lg"
      variant="bordered"
      radius="sm"
    >
      {status || "Open"}
    </Chip>
  );

  const renderDate = (iso: string) => {
    const ts = Date.parse(iso);
    return (
      <span className="text-default-500">
        {Number.isFinite(ts) ? dateTimeFormatter.format(ts) : "—"}
      </span>
    );
  };

  const renderExpandButton = (id: number) => {
    const isSelected = selectedKey === id;

    if (!requiresLocking) {
      return (
        <Button
          onPress={() => handleView(id)}
          variant="flat"
          size="sm"
          disabled={isSelected}
        >
          {isSelected ? "Viewing" : "View"}
        </Button>
      );
    }

    const somethingSelected = selectedKey !== null;

    if (!somethingSelected) {
      return (
        <Button onPress={() => handleView(id)} variant="flat" size="sm">
          View
        </Button>
      );
    }

    if (isSelected) {
      return (
        <Button variant="flat" size="sm" disabled>
          Viewing
        </Button>
      );
    }

    return (
      <Button variant="flat" size="sm" disabled>
        View
      </Button>
    );
  };

  return (
    <div className="flex w-full flex-row gap-4">
      <Table
        aria-label="Requests Table"
        sortDescriptor={asyncList.sortDescriptor}
        onSortChange={asyncList.sort}
        className="w-full"
        isStriped
        isVirtualized
        selectionMode="single"
        selectedKeys={selectedKeys}
        onSelectionChange={handleSelectionChange}
      >
        <TableHeader>
          <TableColumn key="id" allowsSorting>
            ID
          </TableColumn>
          <TableColumn key="requester">Requester</TableColumn>
          <TableColumn key="status" allowsSorting>
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
          items={tableItems}
          isLoading={asyncList.loadingState === "loading"}
          loadingContent={<Spinner label="Loading..." />}
          emptyContent={asyncList.error ? "Failed to load" : "No requests"}
        >
          {(item) => {
            // include selectedKey in the row key to force re-render when selection changes

            return (
              <TableRow key={String(item.id)}>
                <TableCell>{item.id}</TableCell>
                <TableCell>
                  <User
                    id={item.character_id.toString()}
                    avatarProps={{
                      src: `https://images.evetech.net/characters/${item.character_id}/portrait`,
                    }}
                    name={
                      <Snippet
                        key={`char-${item.character_id}-${item.character_name}`}
                        hideSymbol
                        size="sm"
                        radius="none"
                      >
                        {item.character_name}
                      </Snippet>
                    }
                    className="text-default-600"
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
        // TODO handle clicks on buttons - Locking for admins on view
        // TODO refactor to separate component
        <div className="rounded-medium border p-4">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-medium font-semibold">
              Request #{selectedRequest.id} details
            </h3>
            <div className="flex gap-2">
              {selectedRequest.character_id === character_id && (
                <Button
                  variant="ghost"
                  color="danger"
                  onPress={() =>
                    handleRequestButtonClick(
                      "cancel",
                      selectedRequest.id,
                      asyncList
                    )
                  }
                >
                  Cancel
                </Button>
              )}
              {auth_level >= 2 && (
                <Fragment>
                  <Button
                    variant="ghost"
                    color="success"
                    onPress={() =>
                      handleRequestButtonClick(
                        "complete",
                        selectedRequest.id,
                        asyncList
                      )
                    }
                  >
                    Complete
                  </Button>
                  <Button
                    variant="ghost"
                    color="danger"
                    onPress={() =>
                      handleRequestButtonClick(
                        "reject",
                        selectedRequest.id,
                        asyncList
                      )
                    }
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
                      name={getNameById(blueprint.type_id)}
                    />
                  </TableCell>
                  <TableCell>{blueprint?.material_efficiency ?? 0}</TableCell>
                  <TableCell>{blueprint?.time_efficiency ?? 0}</TableCell>
                  <TableCell>{blueprint.runs}</TableCell>
                  <TableCell>{blueprint?.quantity ?? 1}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}

async function handleRequestButtonClick(
  action: string,
  requestId: number,
  itemList: AsyncListData<BlueprintRequest>
) {
  switch (action) {
    case "cancel":
      // Handle cancel action
      try {
        const response = await fetch(`/api/requisition/${requestId}/cancel`, {
          method: "PATCH",
        });
        if (!response.ok) {
          throw new Error(`Failed to cancel request${response.status}`);
        }
        addToast({
          title: "Cancelled",
          description: `Successfully cancelled request ${requestId}`,
          color: "success",
        });
      } catch (err) {
        console.error(err);
        addToast({
          title: "Cancel error",
          description: `Failed to cancel request ${requestId}`,
          color: "danger",
        });
      }

      break;
    case "complete":
      // Handle complete action
      try {
        const response = await fetch(`/api/requisition/${requestId}/complete`, {
          method: "PATCH",
        });
        if (!response.ok) {
          throw new Error(`Failed to cancel request${response.status}`);
        }

        addToast({
          title: "Completed",
          description: `Successfully completed request ${requestId}`,
          color: "success",
        });
      } catch (err) {
        console.error(err);
        addToast({
          title: "Complete error",
          description: `Failed to complete request ${requestId}`,
          color: "danger",
        });
      }

      break;
    case "reject":
      // Handle reject action
      try {
        const response = await fetch(`/api/requisition/${requestId}/reject`, {
          method: "PATCH",
        });
        if (!response.ok) {
          throw new Error(`Failed to cancel request${response.status}`);
        }
        addToast({
          title: "Rejected",
          description: `Successfully rejected request ${requestId}`,
          color: "success",
        });
      } catch (err) {
        console.error(err);
        addToast({
          title: "Reject error",
          description: `Failed to reject request ${requestId}`,
          color: "danger",
        });
      }

      break;
    default:
      break;
  }

  itemList.reload();
}

export default RouteComponent;
