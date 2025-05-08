## Brave Blueprint Programme

From https://wiki.bravecollective.com/public/alliance/industry/bpcprogram

- Brave Alliance only
- 10 BPCs per request
  - Requests for multiple of the same BPC is at the discretion of the BPC manager.
- Only the BPCs on the list can be requested.
- Please only have one request active at a time
  - **Per person NOT per character**
- Capital hull BPCs are provided at one per month per person due to scarcity (and because if you need them, you're generally wealthy enough to be able to get them from contracts)
- The program operates only from "E3OI-U - Librarian"

Anything that is unreasonable will be rejected. And this is a sliding scale, arbitrary and utterly at the discretion of the BPC Managers. If they blink at the request, they’ll reject it.

## Development

Create a development app at [developers.eveonline.com](https://developers.eveonline.com) with the callback URL `http://localhost:2727/login` and following scopes:
- esi-assets.read_corporation_assets.v1
- esi-corporations.read_blueprints.v1
- esi-industry.read_corporation_jobs.v1

Then create `backend/.env` with 
``` sh
ESI_APP_ID=<appid>
ESI_APP_SECRET=<secret>
ESI_APP_REDIRECT=http://localhost:2727/login
```

The backend container can now be built and run using
``` sh
docker compose up -d --build backend
```

Access to npm can be acquired via the node container in the `/app` directory
``` sh
docker compose run node bash
```

Running the frontend container provides a hotloading webserver at `localhost:3000` for react/frontend development.
``` sh
docker compose up -d frontend
```

### TODO
- [ ] Backend
  - [x] Oauth for ESI
    - [x] Corp Blueprint roles to fetch blueprints store in token table
  - [x] Pull Blueprint data via ESI
- [ ] Pages
  - [ ] Unauthenticated / Login Page
  - [ ] Unauthorized Character
  - [ ] Authorized Character
    - [ ] User Settings P2
    - [ ] User Requests
    - [ ] Home Page
  - [ ] Authorized Manager Page
    - [ ] Request Queue
    - [ ] Request History P2
    - [ ] Research Queue
  - [ ] Authorized Admin Page
    - [ ] System Settings
