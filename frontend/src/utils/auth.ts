export function buildLoginHref(currentHref = window.location.href): string {
  const sourceUrl = new URL(currentHref, window.location.href);
  const loginUrl = new URL("/login", sourceUrl);
  loginUrl.searchParams.set("src", sourceUrl.toString());

  return loginUrl.toString();
}

export function redirectToLogin(currentHref = window.location.href): string {
  const loginHref = buildLoginHref(currentHref);
  window.location.assign(loginHref);

  return loginHref;
}
