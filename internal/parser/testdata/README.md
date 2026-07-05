# Fixtures

Captured 2026-07-05 from autoscout24.de via `go run ./cmd/capture-fixtures <url> <name>.html`.

- `search_page.html` — https://www.autoscout24.de/lst/bmw/320?pricefrom=10000&priceto=30000
- `listing_page.html` — /angebote/bmw-320-d-xdrive-mild-hybrid-aut-diesel-cat_ma13mo1641-37d4ccc4-6999-48ef-a92a-2c37232f177d
- `dealer_page.html` — /haendler/auto-kreher-gmbh?atype=C

All pages embed `<script id="__NEXT_DATA__" type="application/json">…</script>`.

## Search page (`props.pageProps`)

- `listings[]` — 20 per page
  - `id` (guid), `url` (relative `/angebote/...`)
  - `price.priceRaw` (int EUR), `price.priceEvaluation` (int category 0..4)
  - `vehicle.{make,model,modelVersionInput,fuel,transmission,mileageInKm(formatted string),offerType}`
  - `location.{countryCode,zip,city,street}`
  - `seller.{id,type("Dealer"|"Private"),companyName,links.infoPage}`
  - `tracking.{firstRegistration("MM-YYYY"),mileage("74983"),price("20000"),priceLabel("toolow-price ")}`
  - `vehicleDetails[]` — localized display rows; power row `data:"140 kW (190 PS)"`, iconName `speedometer`
- `numberOfResults` (int), `numberOfPages` (int), `pageQuery` (echo of query)
- `taxonomy.makes` — map[idString]{label, value(int)}
- `taxonomy.models` — map[makeIdString][]{value(int), label, makeId, modelLineId}

## Listing page (`props.pageProps.listingDetails`)

- `id`, `description` (HTML string), `images[]` (url strings)
- `prices.public.{priceRaw,category(int),median(int),evaluationRanges[]}`
- `vehicle.{make,model,modelVersionInput,firstRegistrationDateRaw("YYYY-MM-DD"),mileageInKmRaw(int),rawPowerInKw(int),primaryFuel.formatted,transmissionType,bodyType,bodyColor,numberOfDoors,numberOfSeats,noOfPreviousOwners,equipment(map, may be empty)}`
- `location.{countryCode,zip,city,street,latitude,longitude}`
- `seller.{id,type,companyName,contactName,links.infoPage,phones[]}`

Price evaluation categories (from evaluationRanges): 0=very good … higher=worse; map ints to labels in parser.

## Dealer page (`props.pageProps`)

- `dealerInfoPage.{customerId,customerName,customerAddress{country,zipCode,city,street},ratings{ratingAverage,ratingCount},homepageUrl,slug,aboutUs}`
- `listings[]` — reduced shape: `id, url, prices.public.priceRaw, vehicle.{make,model,modelVersionInput,firstRegistrationDate,mileageInKm,powerInKw,primaryFuel,transmissionType}, location, seller`
- `numberOfResults`

## URL query params (search)

Confirmed via `pageQuery` echo: `pricefrom, priceto, sort, desc, cy` (country, e.g. `D`). Standard AS24 list params used by the service layer: `kmfrom, kmto, fregfrom, fregto, powerfrom, powerto, powertype=kw, fuel, gear, body, zip, zipr, page`. `priceLabel` values look like `toolow-price`, `good-price`, etc. (trailing space observed — trim).
