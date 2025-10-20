"use client";
import { createFileRoute } from "@tanstack/react-router";
import { ChangeEvent, useEffect, useMemo, useState } from "react";
import {
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
  Input,
  Checkbox,
  Avatar,
  NumberInput,
  Button,
  addToast,
  Spinner,
} from "@heroui/react";
import {
  useBlueprintsQuery,
  type Blueprint,
  type BlueprintGroup,
} from "../api/blueprints";
import { useCreateRequisitionMutation } from "../api/requisitions";

export const Route = createFileRoute("/_auth/list")({
  component: RouteComponent,
});

const MAX_TOTAL = 10;

type SelectedState = Record<string, { checked: boolean; value: number }>;
type BlueprintWithName = Blueprint & { type_name: string };

function RouteComponent() {
  const { data: blueprintGroups = [], isLoading, error } = useBlueprintsQuery();
  const createRequisition = useCreateRequisitionMutation();

  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<keyof Blueprint | "name">("name");
  const [sortAsc, setSortAsc] = useState(true);
  const [selected, setSelected] = useState<SelectedState>({});

  useEffect(() => {
    if (!error) return;
    addToast({
      title: "Error",
      description:
        error instanceof Error
          ? `Failed to load blueprints: ${error.message}`
          : "Failed to load blueprints.",
      color: "danger",
    });
  }, [error]);

  const getTotal = (state: SelectedState = selected) =>
    Object.values(state)
      .filter((entry) => entry.checked)
      .reduce((sum, entry) => sum + entry.value, 0);

  const flattenedBlueprints = useMemo<BlueprintWithName[]>(() => {
    return blueprintGroups.flatMap((group) =>
      group.blueprints.map((blueprint) => ({
        ...blueprint,
        type_name: group.type_name,
      }))
    );
  }, [blueprintGroups]);

  const selectedItems = useMemo(() => {
    return flattenedBlueprints.filter((item) => selected[item.key]?.checked);
  }, [flattenedBlueprints, selected]);

  const toggleExpand = (groupId: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(groupId)) {
        next.delete(groupId);
      } else {
        next.add(groupId);
      }
      return next;
    });
  };

  const handleSort = (key: keyof Blueprint | "name") => {
    if (key === sortKey) {
      setSortAsc((prev) => !prev);
    } else {
      setSortKey(key);
      setSortAsc(true);
    }
  };

  const handleCheck = (key: string, checked: boolean) => {
    setSelected((prev) => {
      const current = prev[key]?.value ?? 1;
      const nextState: SelectedState = {
        ...prev,
        [key]: {
          checked,
          value: current,
        },
      };

      if (checked && getTotal(nextState) > MAX_TOTAL) {
        addToast({
          title: "Selection limit exceeded",
          description: `You cannot exceed the overall limit of ${MAX_TOTAL} BPCs.`,
          color: "danger",
        });
        return prev;
      }

      return nextState;
    });
  };

  const handleValueChange = (item: Blueprint, rawValue: number | null) => {
    const parsedValue = typeof rawValue === "number" ? rawValue : 1;
    const clampedValue = Math.min(
      Math.max(Math.floor(parsedValue) || 1, 1),
      item.quantity
    );

    setSelected((prev) => {
      const nextState: SelectedState = {
        ...prev,
        [item.key]: {
          checked: true,
          value: clampedValue,
        },
      };

      if (getTotal(nextState) > MAX_TOTAL) {
        addToast({
          title: "Selection limit exceeded",
          description: `You cannot exceed the overall limit of ${MAX_TOTAL} BPCs.`,
          color: "danger",
        });
        return prev;
      }

      return nextState;
    });
  };

  const filteredGroups = useMemo<BlueprintGroup[]>(() => {
    if (blueprintGroups.length === 0) {
      return [];
    }

    const normalizedSearch = search.trim().toLowerCase();
    const filtered = normalizedSearch.length
      ? blueprintGroups.filter((group) =>
          group.type_name.toLowerCase().includes(normalizedSearch)
        )
      : blueprintGroups;

    const sorted = [...filtered].sort((a, b) => {
      if (sortKey === "name") {
        return a.type_name.localeCompare(b.type_name) * (sortAsc ? 1 : -1);
      }

      const valueA = a.blueprints[0]?.[sortKey];
      const valueB = b.blueprints[0]?.[sortKey];

      if (valueA == null) return 1;
      if (valueB == null) return -1;
      return (valueA > valueB ? 1 : -1) * (sortAsc ? 1 : -1);
    });

    return sorted;
  }, [blueprintGroups, search, sortAsc, sortKey]);

  const handleSubmit = async () => {
    if (selectedItems.length === 0) {
      addToast({
        title: "No items selected",
        description: "Select at least one blueprint before submitting.",
        color: "warning",
      });
      return;
    }

    try {
      await createRequisition.mutateAsync({
        blueprints: selectedItems.map((item) => ({
          type_id: item.type_id,
          runs: item.runs,
          material_efficiency: item.material_efficiency ?? 0,
          time_efficiency: item.time_efficiency ?? 0,
          quantity: selected[item.key]?.value ?? item.quantity,
        })),
      });
      addToast({
        title: "Success",
        description: "Requisition created successfully!",
        color: "success",
      });
      setSelected({});
    } catch (err) {
      addToast({
        title: "Error",
        description:
          err instanceof Error ? err.message : "Failed to create requisition.",
        color: "danger",
      });
    }
  };

  return (
    <div className="flex gap-4">
      <div>
        <Input
          placeholder="Search items..."
          value={search}
          onChange={(event: ChangeEvent<HTMLInputElement>) =>
            setSearch(event.target.value)
          }
          className="font-semibold w-64 p-2"
        />

        <Table
          className="flex gap-4"
          aria-label="Collapsible selectable table"
          isStriped
          isVirtualized
        >
          <TableHeader>
            <TableColumn
              onClick={() => handleSort("name")}
              className="cursor-pointer"
            >
              Name
            </TableColumn>
            <TableColumn>Group</TableColumn>
            <TableColumn className="cursor-pointer">ME</TableColumn>
            <TableColumn className="cursor-pointer">TE</TableColumn>
            <TableColumn className="cursor-pointer">Runs</TableColumn>
            <TableColumn className="cursor-pointer">Quantity</TableColumn>
            <TableColumn>Adjust</TableColumn>
            <TableColumn>Select</TableColumn>
          </TableHeader>
          <TableBody
            isLoading={isLoading}
            loadingContent={<Spinner label="Loading..." />}
            emptyContent="No data"
          >
            {filteredGroups.flatMap((group) => {
              const { type_name: typeName, blueprints } = group;
              const showExpandable = blueprints.length > 1;

              if (showExpandable) {
                const headerRow = (
                  <TableRow
                    key={`${typeName}-header`}
                    onClick={() => toggleExpand(typeName)}
                    className="cursor-pointer"
                  >
                    <TableCell className="flex items-center gap-2 font-semibold">
                      <Avatar
                        radius="none"
                        src={`https://images.evetech.net/types/${blueprints[0].type_id}/bpc`}
                      />
                      {typeName}
                    </TableCell>
                    <TableCell colSpan={5} className="text-sm text-gray-500">
                      {expandedGroups.has(typeName)
                        ? "Click to collapse"
                        : "Click to expand"}
                    </TableCell>
                    <TableCell>{null}</TableCell>
                    <TableCell>{null}</TableCell>
                  </TableRow>
                );

                if (!expandedGroups.has(typeName)) {
                  return [headerRow];
                }

                const detailRows = blueprints.map((blueprint) => {
                  const state = selected[blueprint.key];
                  return (
                    <TableRow key={blueprint.key}>
                      <TableCell>{null}</TableCell>
                      <TableCell>{null}</TableCell>
                      <TableCell>
                        {blueprint.material_efficiency ?? 0}
                      </TableCell>
                      <TableCell>{blueprint.time_efficiency ?? 0}</TableCell>
                      <TableCell>{blueprint.runs}</TableCell>
                      <TableCell>{blueprint.quantity}</TableCell>
                      <TableCell>
                        {state?.checked && (
                          <NumberInput
                            size="sm"
                            className="w-20 mt-1"
                            minValue={1}
                            maxValue={blueprint.quantity}
                            value={state.value}
                            onValueChange={(value) =>
                              handleValueChange(blueprint, value)
                            }
                          />
                        )}
                      </TableCell>
                      <TableCell>
                        <Checkbox
                          isSelected={state?.checked ?? false}
                          onValueChange={(checked) =>
                            handleCheck(blueprint.key, checked)
                          }
                        />
                      </TableCell>
                    </TableRow>
                  );
                });

                return [headerRow, ...detailRows];
              }

              return blueprints.map((blueprint) => {
                const state = selected[blueprint.key];
                return (
                  <TableRow key={blueprint.key}>
                    <TableCell className="flex items-center gap-2 font-semibold">
                      <Avatar
                        radius="none"
                        src={`https://images.evetech.net/types/${blueprint.type_id}/bpc`}
                      />
                      {typeName}
                    </TableCell>
                    <TableCell>{null}</TableCell>
                    <TableCell>{blueprint.material_efficiency ?? 0}</TableCell>
                    <TableCell>{blueprint.time_efficiency ?? 0}</TableCell>
                    <TableCell>{blueprint.runs}</TableCell>
                    <TableCell>{blueprint.quantity}</TableCell>
                    <TableCell>
                      {state?.checked && (
                        <NumberInput
                          size="sm"
                          className="w-20 mt-1"
                          minValue={1}
                          maxValue={blueprint.quantity}
                          value={state.value}
                          onValueChange={(value) =>
                            handleValueChange(blueprint, value)
                          }
                        />
                      )}
                    </TableCell>
                    <TableCell>
                      <Checkbox
                        isSelected={state?.checked ?? false}
                        onValueChange={(checked) =>
                          handleCheck(blueprint.key, checked)
                        }
                      />
                    </TableCell>
                  </TableRow>
                );
              });
            })}
          </TableBody>
        </Table>
      </div>
      {selected && (
        <div className="flex gap-4 flex-col border rounded-lg p-4">
          <h2 className="text-center font-bold">Selected Items</h2>

          <Table className="flex-1 h-full" aria-label="Selected items table">
            <TableHeader>
              <TableColumn>Name</TableColumn>
              <TableColumn>Quantity</TableColumn>
              <TableColumn>Runs</TableColumn>
              <TableColumn>ME</TableColumn>
              <TableColumn>TE</TableColumn>
            </TableHeader>
            <TableBody emptyContent="No items selected">
              {selectedItems.map((item) => {
                const quantity = selected[item.key]?.value ?? 1;
                return (
                  <TableRow key={`${item.key}-selected`}>
                    <TableCell>
                      <Avatar
                        radius="none"
                        src={`https://images.evetech.net/types/${item.type_id}/bpc`}
                      />
                      {item.type_name || "Unknown Item"}
                    </TableCell>
                    <TableCell>{quantity}</TableCell>
                    <TableCell>{item.runs}</TableCell>
                    <TableCell>{item.material_efficiency ?? 0}</TableCell>
                    <TableCell>{item.time_efficiency ?? 0}</TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
          <Button
            className="object-bottom"
            onPress={() => void handleSubmit()}
            isDisabled={
              selectedItems.length === 0 || createRequisition.isPending
            }
            isLoading={createRequisition.isPending}
          >
            Submit
          </Button>
        </div>
      )}
    </div>
  );
}

export default RouteComponent;
