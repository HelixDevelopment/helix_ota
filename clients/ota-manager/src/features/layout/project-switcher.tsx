import { useState } from "react";
import { Check, ChevronsUpDown, FolderKanban } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const MOCK_PROJECTS = [
  { id: "1", name: "ATMOSphere", slug: "atmosphere" },
  { id: "2", name: "Helix OTA", slug: "helix-ota" },
];

export function ProjectSwitcher() {
  const [selected, setSelected] = useState(MOCK_PROJECTS[0]);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" className="w-full justify-between px-2">
          <div className="flex items-center gap-2 truncate">
            <FolderKanban className="h-4 w-4 shrink-0" />
            <span className="text-sm font-medium truncate">{selected?.name}</span>
          </div>
          <ChevronsUpDown className="h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-[200px]">
        <DropdownMenuLabel>Projects</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {MOCK_PROJECTS.map((project) => (
          <DropdownMenuItem
            key={project.id}
            onClick={() => setSelected(project)}
            className="cursor-pointer"
          >
            <Check
              className={`mr-2 h-4 w-4 ${
                selected?.id === project.id ? "opacity-100" : "opacity-0"
              }`}
            />
            {project.name}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
