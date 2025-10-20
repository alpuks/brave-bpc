import {
  Avatar,
  Checkbox,
  NumberInput,
  Spinner,
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
} from "@heroui/react";
import { memo } from "react";
import type { Blueprint, BlueprintGroup } from "../api/blueprints";

export type BlueprintSelectionState = Record<
  string,
  { checked: boolean; value: number }
>;

export interface BlueprintGroupsTableProps {
  groups: BlueprintGroup[];
  expandedGroups: Set<string>;
  onToggleGroup: (groupId: string) => void;
  onSort: (key: keyof Blueprint | "name") => void;
  sortKey: keyof Blueprint | "name";
  sortAsc: boolean;
  selectionState: BlueprintSelectionState;
  onValueChange: (blueprint: Blueprint, value: number | null) => void;
  onCheck: (key: string, checked: boolean) => void;
  isLoading: boolean;
}

const BlueprintGroupsTable = memo(
  ({
    groups,
    expandedGroups,
    onToggleGroup,
    onSort,
    sortKey,
    sortAsc,
    selectionState,
    onValueChange,
    onCheck,
    isLoading,
  }: BlueprintGroupsTableProps) => {
    const renderSortIndicator = (key: keyof Blueprint | "name") =>
      sortKey === key ? (sortAsc ? " ▲" : " ▼") : "";

    const renderDetailRow = (
      blueprint: Blueprint,
      typeName: string,
      showTypeInfo: boolean
    ) => {
      const state = selectionState[blueprint.key];
      return (
        <TableRow key={blueprint.key}>
          <TableCell
            className={
              showTypeInfo ? "flex items-center gap-2 font-semibold" : undefined
            }
          >
            {showTypeInfo ? (
              <>
                <Avatar
                  radius="none"
                  src={`https://images.evetech.net/types/${blueprint.type_id}/bpc`}
                />
                {typeName}
              </>
            ) : null}
          </TableCell>
          <TableCell>{null}</TableCell>
          <TableCell>{blueprint.material_efficiency ?? 0}</TableCell>
          <TableCell>{blueprint.time_efficiency ?? 0}</TableCell>
          <TableCell>{blueprint.runs}</TableCell>
          <TableCell>{blueprint.quantity}</TableCell>
          <TableCell>
            {state?.checked && (
              <NumberInput
                className="w-20 mt-1"
                maxValue={blueprint.quantity}
                minValue={1}
                onValueChange={(value) => onValueChange(blueprint, value)}
                size="sm"
                value={state.value}
              />
            )}
          </TableCell>
          <TableCell>
            <Checkbox
              isSelected={state?.checked ?? false}
              onValueChange={(checked) => onCheck(blueprint.key, checked)}
            />
          </TableCell>
        </TableRow>
      );
    };

    return (
      <Table
        aria-label="Collapsible selectable table"
        className="flex gap-4"
        isStriped
        isVirtualized
      >
        <TableHeader>
          <TableColumn
            className="cursor-pointer"
            onClick={() => onSort("name")}
          >
            Name
            {renderSortIndicator("name")}
          </TableColumn>
          <TableColumn>Group</TableColumn>
          <TableColumn
            className="cursor-pointer"
            onClick={() => onSort("material_efficiency")}
          >
            ME
            {renderSortIndicator("material_efficiency")}
          </TableColumn>
          <TableColumn
            className="cursor-pointer"
            onClick={() => onSort("time_efficiency")}
          >
            TE
            {renderSortIndicator("time_efficiency")}
          </TableColumn>
          <TableColumn
            className="cursor-pointer"
            onClick={() => onSort("runs")}
          >
            Runs
            {renderSortIndicator("runs")}
          </TableColumn>
          <TableColumn
            className="cursor-pointer"
            onClick={() => onSort("quantity")}
          >
            Quantity
            {renderSortIndicator("quantity")}
          </TableColumn>
          <TableColumn>Adjust</TableColumn>
          <TableColumn>Select</TableColumn>
        </TableHeader>
        <TableBody
          emptyContent="No data"
          isLoading={isLoading}
          loadingContent={<Spinner label="Loading..." />}
        >
          {groups.flatMap((group) => {
            const { type_name: typeName, blueprints } = group;
            const firstBlueprint = blueprints[0];

            if (!firstBlueprint) {
              return [];
            }

            const showExpandable = blueprints.length > 1;

            if (!showExpandable) {
              return blueprints.map((blueprint) =>
                renderDetailRow(blueprint, typeName, true)
              );
            }

            const headerRow = (
              <TableRow
                key={`${typeName}-header`}
                className="cursor-pointer"
                onClick={() => onToggleGroup(typeName)}
              >
                <TableCell className="flex items-center gap-2 font-semibold">
                  <Avatar
                    radius="none"
                    src={`https://images.evetech.net/types/${firstBlueprint.type_id}/bpc`}
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

            const detailRows = blueprints.map((blueprint) =>
              renderDetailRow(blueprint, typeName, false)
            );

            return [headerRow, ...detailRows];
          })}
        </TableBody>
      </Table>
    );
  }
);

BlueprintGroupsTable.displayName = "BlueprintGroupsTable";

export default BlueprintGroupsTable;
