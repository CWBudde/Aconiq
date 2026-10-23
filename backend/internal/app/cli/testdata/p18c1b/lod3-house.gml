<?xml version="1.0" encoding="UTF-8"?>
<!-- A synthetic LoD3 house (P18 C1b): 10 m x 8 m on ground at z = 50, 9 m
     tall, at invented coordinates (its south-west corner at 500000, 5800000) and a synthetic address. Its south wall
     (y = 5800000) carries a window recessed 0.2 m into its reveal and a door, its
     east wall (x = 500010) a window, its roof a skylight; one window lies inside
     the house, in no facade plane. -->
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
  xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
  xmlns:gml="http://www.opengis.net/gml"
  xmlns:xAL="urn:oasis:names:tc:ciq:xsdschema:xAL:2.0">
  <core:cityObjectMember>
    <bldg:Building gml:id="lod3-house">
      <bldg:measuredHeight uom="urn:adv:uom:m">9</bldg:measuredHeight>
      <bldg:boundedBy>
        <bldg:GroundSurface gml:id="ground">
          <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500000 5800000 50 500000 5800008 50 500010 5800008 50 500010 5800000 50 500000 5800000 50</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
        </bldg:GroundSurface>
      </bldg:boundedBy>
      <bldg:boundedBy>
        <bldg:WallSurface gml:id="wall-south">
          <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500000 5800000 50 500010 5800000 50 500010 5800000 59 500000 5800000 59 500000 5800000 50</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
          <bldg:opening>
            <bldg:Window gml:id="w-south">
              <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500002 5800000.2 53.9 500003.2 5800000.2 53.9 500003.2 5800000.2 55.4 500002 5800000.2 55.4 500002 5800000.2 53.9</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
            </bldg:Window>
          </bldg:opening>
          <bldg:opening>
            <bldg:Door gml:id="door-1">
              <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500005 5800000 50 500006 5800000 50 500006 5800000 52.1 500005 5800000 52.1 500005 5800000 50</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
            </bldg:Door>
          </bldg:opening>
        </bldg:WallSurface>
      </bldg:boundedBy>
      <bldg:boundedBy>
        <bldg:WallSurface gml:id="wall-east">
          <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500010 5800000 50 500010 5800008 50 500010 5800008 59 500010 5800000 59 500010 5800000 50</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
          <bldg:opening>
            <bldg:Window gml:id="w-east">
              <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500010 5800003 51 500010 5800004 51 500010 5800004 52.2 500010 5800003 52.2 500010 5800003 51</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
            </bldg:Window>
          </bldg:opening>
        </bldg:WallSurface>
      </bldg:boundedBy>
      <bldg:boundedBy>
        <bldg:WallSurface gml:id="wall-inner">
          <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500000 5800003 50 500010 5800003 50 500010 5800003 59 500000 5800003 59 500000 5800003 50</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
          <bldg:opening>
            <bldg:Window gml:id="w-adrift">
              <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500004 5800003 51 500005 5800003 51 500005 5800003 52 500004 5800003 52 500004 5800003 51</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
            </bldg:Window>
          </bldg:opening>
        </bldg:WallSurface>
      </bldg:boundedBy>
      <bldg:boundedBy>
        <bldg:RoofSurface gml:id="roof">
          <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500000 5800000 59 500010 5800000 59 500010 5800008 59 500000 5800008 59 500000 5800000 59</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
          <bldg:opening>
            <bldg:Window gml:id="skylight">
              <bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember><gml:Polygon><gml:exterior><gml:LinearRing><gml:posList>500004 5800004 59 500005 5800004 59 500005 5800005 59 500004 5800005 59 500004 5800004 59</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon></gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>
            </bldg:Window>
          </bldg:opening>
        </bldg:RoofSurface>
      </bldg:boundedBy>
      <bldg:address>
        <core:Address>
          <core:xalAddress>
            <xAL:AddressDetails>
              <xAL:Country>
                <xAL:Locality>
                  <xAL:Thoroughfare>
                    <xAL:ThoroughfareNumber>1</xAL:ThoroughfareNumber>
                    <xAL:ThoroughfareName>Musterstraße</xAL:ThoroughfareName>
                  </xAL:Thoroughfare>
                </xAL:Locality>
              </xAL:Country>
            </xAL:AddressDetails>
          </core:xalAddress>
        </core:Address>
      </bldg:address>
    </bldg:Building>
  </core:cityObjectMember>
</core:CityModel>
