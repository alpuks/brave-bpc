import {
  Button,
  Chip,
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
  Spinner,
  User,
  Snippet,
} from "@heroui/react";
import type { Selection, SortDescriptor } from "@react-types/shared";
import { memo } from "react";
import type { BlueprintRequest } from "../api/requisitions";

export type StatusMetadata = {
  value: number;
  slug: string;
  name: string;
  color: "default" | "primary" | "secondary" | "success" | "warning" | "danger";
};

export interface RequestsTableProps {
  items: BlueprintRequest[];
  error?: unknown;
  isLoading: boolean;
  selectedKeys: Selection;
  sortDescriptor: SortDescriptor;
  selectedKey: number | null;
  selectedRequest: BlueprintRequest | null;
  shouldLockRequest: (request?: BlueprintRequest | null) => boolean;
  resolveStatusMetadata: (status?: string | number) => StatusMetadata;
  formatDate: (iso: string) => string;
  onView: (id: number) => void;
  onSelectionChange: (keys: Selection | "all") => void;
  onSortChange: (descriptor: SortDescriptor) => void;
}

const RequestsTable = memo(
  ({
    items,
    error,
    isLoading,
    selectedKeys,
    sortDescriptor,
    selectedKey,
    selectedRequest,
    shouldLockRequest,
    resolveStatusMetadata,
    formatDate,
    onView,
    onSelectionChange,
    onSortChange,
  }: RequestsTableProps) => (
    <Table
      aria-label="Requests Table"
      className="w-full"
      isStriped
      isVirtualized
      onSelectionChange={onSelectionChange}
      onSortChange={onSortChange}
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
        isLoading={isLoading}
        items={items}
        loadingContent={<Spinner label="Loading..." />}
      >
        {(item: BlueprintRequest) => {
          const characterName = item.character_name || "Unknown";

          const renderStatus = () => {
            const metadata = resolveStatusMetadata(item.status);
            return (
              <Chip
                className="capitalize gap-1 text-default-600 text-bold"
                color={metadata.color}
                radius="sm"
                size="lg"
                variant="bordered"
              >
                {metadata.name}
              </Chip>
            );
          };

          const renderDate = (iso: string) => {
            const formatted = formatDate(iso);
            return <span className="text-default-500">{formatted}</span>;
          };

          const renderExpandButton = () => {
            const { id } = item;
            const isSelected = selectedKey === id;
            const requestNeedsLock = shouldLockRequest(item);
            const selectedNeedsLock = shouldLockRequest(selectedRequest);

            if (!requestNeedsLock) {
              const disabled = isSelected || selectedNeedsLock;
              return (
                <Button
                  disabled={disabled}
                  onPress={() => onView(id)}
                  size="sm"
                  variant="flat"
                >
                  {isSelected ? "Viewing" : "View"}
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

            if (!selectedNeedsLock) {
              return (
                <Button onPress={() => onView(id)} size="sm" variant="flat">
                  View
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
            <TableRow key={String(item.id)}>
              <TableCell>{item.id}</TableCell>
              <TableCell>
                <User
                  avatarProps={{
                    src: `https://images.evetech.net/characters/${item.character_id}/portrait?size=128`,
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
              <TableCell>{renderStatus()}</TableCell>
              <TableCell>{renderDate(item.created_at)}</TableCell>
              <TableCell>{renderDate(item.updated_at)}</TableCell>
              <TableCell>{item.updated_by || "N/A"}</TableCell>
              <TableCell>{item.public_notes}</TableCell>
              <TableCell>{renderExpandButton()}</TableCell>
            </TableRow>
          );
        }}
      </TableBody>
    </Table>
  )
);

RequestsTable.displayName = "RequestsTable";

export default RequestsTable;
