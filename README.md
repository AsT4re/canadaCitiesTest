## cancities ##

# Description #

Simple HTTP REST server using go as language and DGraph as database for storing locations (here representing cities in Canada).

This server has 3 main APIs

- a POST request `/import`

  For importing a Geo JSON file in DB. The file is a GeoJson file compliant with the format described at http://geojson.org/

  It is structured as :

  ```
  {
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "Point",
        "coordinates": [
          -80.643498,
          43.069946
        ]
      },
      "properties": {
        "name": "Oriel",
        "place_key": "3500002520",
        "capital": "N",
        "population": 2500,
        "pclass": "2",
        "cartodb_id": 744,
        "created_at": "2015-04-02T23:52:39Z",
        "updated_at": "2015-04-02T23:52:39Z"
      }
    },
    { another feature (city)... }
    ]
  }
  ```

  Example:
  ```
  curl -ks -XPOST 'https://localhost:8443/import' -d @data/canada_cities.geojson.txt
  ```

- a GET request `/id/<12345>`

  Returns the city in DB which have the given `id`

  Example:
  ```
  curl -ks https://localhost:8443/id/744
  {
    "cartodb_id": 744,
    "name": "oriel",
    "population": 2500,
    "coordinates": [-80.643498,43.069946]
  }
  ```

- a GET request `/id/<12345>?dist=10`

  Returns all the cities in DB contained in a square where side has a size of `dist` (in Kilometers) and where center is the location of the city with given `id`

   Example :
   ```
   curl -ks https://localhost:8443/id/744?dist=10

   [
     {
       "cartodb_id": 544,
       "name": "Domaine-Pacha",
       "population": 29500,
       "coordinates": [-80.643497,43.069947]
     },
     {
       "cartodb_id": 998,
       "name": "Rhodena",
       "population": 210,
       "coordinates": [-80.643478,43.069947]
     },
      ...
   ]
   ```

# Install requirements #

- Launch dgraph

  ```
  docker pull dgraph/dgraph:v0.8.3
  docker run -it -p 8080:8080 -p 9080:9080 -v ~/dgraph:/dgraph --name dgraph dgraph/dgraph:v0.8.3 dgraphzero -w zw
  docker exec -it dgraph dgraph --bindall=true --memory_mb 8192 -peer 127.0.0.1:8888
  ```

- Get and start server

  ```
  go get github.com/AsT4re/cancities
  cancities --tls-crt $GOPATH/src/github.com/AsT4re/cancities/certificates/server.crt --tls-key $GOPATH/src/github.com/AsT4re/cancities/certificates/server.key
  ```

- Import geo datas

  ```
  curl -ks -XPOST 'https://localhost:8443/import' -d @$GOPATH/src/github.com/AsT4re/cancities/data/canada_cities.geojson.txt
  ```

- Send requests

  ```
  curl -ks https://localhost:8443/id/744
  curl -ks https://localhost:8443/id/744?dist=10
  ```