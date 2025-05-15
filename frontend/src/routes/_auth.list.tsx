import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useState } from "react";
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
  
} from "@heroui/react";

export const Route = createFileRoute("/_auth/list")({
  component: RouteComponent,
});

export interface Blueprint {
	name?:string;
  item_id: number;
  location_flag: string;
  location_id: number;
  quantity: number;
  runs: number;
  type_id: number;
  material_efficiency?: number;
  time_efficiency?: number;
}
const responseData = `{"4410":[{"item_id":1046392647306,"location_flag":"CorpSAG1","location_id":1049281607545,"quantity":1,"runs":3,"type_id":4410},{"item_id":1047447115336,"location_flag":"CorpSAG1","location_id":1049281607545,"quantity":14,"runs":25,"type_id":4410}],"81043":[{"item_id":1049172803948,"location_flag":"CorpSAG1","location_id":1049281607545,"material_efficiency":10,"quantity":2,"runs":1,"time_efficiency":20,"type_id":81043}]}`
function RouteComponent() {

//   const [blueprints, setBlueprints] = useState(new Map<string, Blueprint[]>());
//   const [loading, setLoading] = useState(true);

// lets load once, then do everything else in-memory
	const bpResponse = JSON.parse(responseData)

	const dataMap = new Map<string, Blueprint[]>(Object.entries(bpResponse));
	// what structure I want for easier sorting/grouping
	type ArrayItem = {
		item_id: number
		name: string;
		quantity: number
		runs: number;
		type_id: number
		material_efficiency?: number
		time_efficiency?: number
	}
	// 

  
//   useEffect(() => {
//     setLoading(true);
//     fetch(
//       `${window.location.protocol}//${window.location.hostname}:2727/api/blueprints`,
//       {
//         method: "GET",
//         credentials: "include",
//         mode: "cors",
//       }
//     )
//       .then((response) => response.json())
//       .then((bluePrints) => {
//         let map = new Map(Object.entries(bluePrints));
//         // TODO - FIGURE out which format to use for this whole mess
//         setBlueprints(map);
//       })
//       .finally(() => setLoading(false));
//   }, []);
	  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState('');
  const [sortKey, setSortKey] = useState<keyof ArrayItem>('name');
  const [sortAsc, setSortAsc] = useState(true);
  const [selectedItems, setSelectedItems] = useState<Map<number, number>>(new Map());

	const toggleExpand = (groupId: string) => {
    setExpandedGroups(prev => {
      const next = new Set(prev);
      next.has(groupId) ? next.delete(groupId) : next.add(groupId);
      return next;
    });
  };

  const handleSort = (key: keyof ArrayItem) => {
    if (key === sortKey) {
      setSortAsc(!sortAsc);
    } else {
      setSortKey(key);
      setSortAsc(true);
    }
  };

  const toggleItemSelection = (itemId: number, checked: boolean) => {
    setSelectedItems(prev => {
      const next = new Map(prev);
      if (checked) {
        next.set(itemId, 1);
      } else {
        next.delete(itemId);
      }
      return next;
    });
  };

  const updateItemValue = (itemId: number, value: number) => {
    setSelectedItems(prev => {
      const next = new Map(prev);
      if (next.has(itemId)) {
        next.set(itemId, value);
      }
      return next;
    });
  };
 const filteredGroups = useMemo(() => {
    return Array.from(dataMap.entries()).map(([groupId, items]) => {
    //   const filteredItems = items.filter(item =>
    //     item.name.toLowerCase().includes(search.toLowerCase())
    //   );
	  const filteredItems = items
	  // TODO remove above workaround since there are no names
      const sortedItems = [...filteredItems].sort((a, b) => {
        const valA = a[sortKey];
        const valB = b[sortKey];
        if (valA == null) return 1;
        if (valB == null) return -1;
        return (valA > valB ? 1 : -1) * (sortAsc ? 1 : -1);
      });

      return [groupId, sortedItems] as [string, ArrayItem[]];
    });
  }, [search, sortKey, sortAsc]);






 
  return (
	<div>
		<Input
        placeholder="Search items..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="w-64"
      />
	  <Table aria-label="Collapsible selectable table" isStriped removeWrapper>
        <TableHeader>
          <TableColumn>Group</TableColumn>
          <TableColumn onClick={() => handleSort('name')} className="cursor-pointer">Name</TableColumn>
          <TableColumn className="cursor-pointer">Quantity</TableColumn>
          <TableColumn className="cursor-pointer">Runs</TableColumn>
          <TableColumn>Choose</TableColumn>
        </TableHeader>
        <TableBody>
          {filteredGroups.map(([groupId, items]) => {
            const showExpandable = items.length > 1;

            return (
              <>
                {showExpandable ? (
                  <>
                    <TableRow
                      key={groupId}
                      onClick={() => toggleExpand(groupId)}
                      className="cursor-pointer"
                    >
                      <TableCell className="flex items-center gap-2 font-semibold">
						<Avatar radius="none" src={`https://images.evetech.net/types/${items[0].type_id}/bpc`} />
                        {groupId}
                      </TableCell>
                      <TableCell colSpan={4} className="text-sm text-gray-500">
                        {expandedGroups.has(groupId) ? "Click to collapse" : "Click to expand"}
                      </TableCell>
                    </TableRow>

                    {expandedGroups.has(groupId) &&
                      items.map((item) => (
                        <TableRow key={item.item_id}>
                          <TableCell children={undefined} />
                          <TableCell>{item.name}</TableCell>
                          <TableCell>{item.quantity}</TableCell>
                          <TableCell>{item.runs}</TableCell>
                          <TableCell>
                            <Checkbox
                              isSelected={selectedItems.has(item.item_id)}
                              onValueChange={(checked) => toggleItemSelection(item.item_id, checked)}
                            />
                            {selectedItems.has(item.item_id) && (
                              <Input
                                type="number"
                                size="sm"
                                className="w-20 mt-1"
                                value={selectedItems.get(item.item_id)?.toString() || '1'}
                                onChange={(e) => updateItemValue(item.item_id, Number(e.target.value))}
                              />
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                  </>
                ) : (
                  items.map((item) => (
                    <TableRow key={item.item_id}>
                      <TableCell className="font-semibold"><Avatar radius="none" src={`https://images.evetech.net/types/${item.type_id}/bpc`} />{groupId}</TableCell>
                      <TableCell>{item.name}</TableCell>
                      <TableCell>{item.quantity}</TableCell>
                      <TableCell>{item.runs}</TableCell>
                      <TableCell>
                        <Checkbox
                          isSelected={selectedItems.has(item.item_id)}
                          onValueChange={(checked) => toggleItemSelection(item.item_id, checked)}
                        />
                        {selectedItems.has(item.item_id) && (
                          <Input
                            type="number"
                            size="sm"
                            className="w-20 mt-1"
                            value={selectedItems.get(item.item_id)?.toString() || '1'}
                            onChange={(e) => updateItemValue(item.item_id, Number(e.target.value))}
                          />
                        )}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </>
            );
          })}
        </TableBody>
      </Table>
	</div>

  )
}
