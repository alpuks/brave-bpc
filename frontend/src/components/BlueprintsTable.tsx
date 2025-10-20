import {
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
  User,
} from "@heroui/react";
import { memo } from "react";
import type { BlueprintRequest } from "../api/requisitions";

type BlueprintLineItem = BlueprintRequest["blueprints"][number];

export interface BlueprintsTableProps {
  blueprints: BlueprintLineItem[];
  emptyContent?: string;
  ariaLabel?: string;
  className?: string;
}

const BlueprintsTable = memo(
  ({
    blueprints,

    emptyContent = "No blueprints",
    ariaLabel = "Blueprints",
    className,
  }: BlueprintsTableProps) => (
    <Table aria-label={ariaLabel} className={className}>
      <TableHeader>
        <TableColumn key="bp">Blueprint</TableColumn>
        <TableColumn key="me">ME</TableColumn>
        <TableColumn key="te">TE</TableColumn>
        <TableColumn key="runs">Runs</TableColumn>
        <TableColumn key="qty">Quantity</TableColumn>
      </TableHeader>
      <TableBody emptyContent={emptyContent}>
        {blueprints.map((blueprint) => (
          <TableRow key={blueprint.type_id}>
            <TableCell>
              <User
                avatarProps={{
                  src: `https://images.evetech.net/types/${blueprint.type_id}/bpc`,
                }}
                name={blueprint.type_name}
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
  )
);

BlueprintsTable.displayName = "BlueprintsTable";

export default BlueprintsTable;
