import {
  Navbar,
  NavbarContent,
  NavbarItem,
  Link,
  Dropdown,
  DropdownTrigger,
  Avatar,
  DropdownMenu,
  DropdownItem,
  Image,
} from "@heroui/react";
import { AuthContext } from "../contexts/AuthContext";
import { useRouter } from "@tanstack/react-router";
import blackEveImage from "../assets/eve-sso-login-black-small.png";

export function NavBar({ authContext }: { authContext: AuthContext }) {
  const isAuthenticated = authContext?.isAuthenticated;
  const user = authContext?.user;

  const router = useRouter();

  return (
    <Navbar as="nav" className="bg-gray-800" maxWidth="full">
      <NavbarContent className="hidden sm:flex gap-4" justify="center">
        {isAuthenticated ? (
          <>
            <NavbarItem>
              <Link color="foreground" href="/dashboard">
                Dashboard
              </Link>
            </NavbarItem>
            <NavbarItem>
              <Link color="foreground" href="/list">
                List
              </Link>
            </NavbarItem>
            <NavbarItem>
              <Link color="foreground" href="/requests">
                Requests
              </Link>
            </NavbarItem>
          </>
        ) : (
          <NavbarItem>
            <Link color="foreground" href="/">
              Home
            </Link>
          </NavbarItem>
        )}
        {user?.auth_level === "admin" && (
          <NavbarItem>
            <Link color="foreground" href="/admin">
              Admin
            </Link>
          </NavbarItem>
        )}
      </NavbarContent>
      <NavbarContent as="div" justify="end">
        {isAuthenticated ? (
          <Dropdown placement="bottom-end">
            <DropdownTrigger>
              <Avatar
                isBordered
                as="button"
                className="transition-transform"
                color="secondary"
                name={user?.character_name}
                size="sm"
                src={
                  "https://images.evetech.net/characters/" +
                  user?.character_id +
                  "/portrait"
                }
              />
            </DropdownTrigger>
            <DropdownMenu aria-label="Profile Actions" variant="flat">
              <DropdownItem key="profile" className="h-14 gap-2">
                <p className="font-semibold">Signed in as</p>
                <p className="font-semibold">{user?.character_name}</p>
              </DropdownItem>
              <DropdownItem
                key="logout"
                color="danger"
                onClick={() =>
                  router.navigate({ to: "/" }) && authContext.logout()
                }
              >
                Log Out
              </DropdownItem>
            </DropdownMenu>
          </Dropdown>
        ) : (
          <a
            href={`${window.location.protocol}//${window.location.hostname}:2727/login?src=${window.location.href}`}
          >
            <Image
              radius="none"
              width="135"
              src={window.location.origin + blackEveImage}
              loading="eager"
            />
          </a>
        )}
      </NavbarContent>
    </Navbar>
  );
}
